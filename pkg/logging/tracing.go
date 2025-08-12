package logging

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceManager manages distributed tracing for the application
type TraceManager struct {
	propagator   propagation.TextMapPropagator
	sampler      TraceSampler
	spans        map[string]*SpanInfo
	mu           sync.RWMutex
	config       TracingConfig
	healthStatus TracingHealthStatus
}

// TracingConfig defines the configuration for distributed tracing
type TracingConfig struct {
	Enabled            bool    `yaml:"enabled" json:"enabled"`
	SamplingRate       float64 `yaml:"samplingRate" json:"samplingRate"`
	MaxSpansPerTrace   int     `yaml:"maxSpansPerTrace" json:"maxSpansPerTrace"`
	SpanTimeoutSeconds int     `yaml:"spanTimeoutSeconds" json:"spanTimeoutSeconds"`
	EnableBaggage      bool    `yaml:"enableBaggage" json:"enableBaggage"`
	MaxBaggageSize     int     `yaml:"maxBaggageSize" json:"maxBaggageSize"`
}

// TracingHealthStatus represents the health status of the tracing system
type TracingHealthStatus struct {
	Healthy          bool  `json:"healthy"`
	ActiveSpans      int   `json:"active_spans"`
	TotalSpansCreated int64 `json:"total_spans_created"`
	TotalSpansFinished int64 `json:"total_spans_finished"`
	SpansDropped     int64 `json:"spans_dropped"`
}

// SpanInfo holds information about an active span
type SpanInfo struct {
	TraceID      string                 `json:"trace_id"`
	SpanID       string                 `json:"span_id"`
	ParentID     string                 `json:"parent_id,omitempty"`
	Operation    string                 `json:"operation"`
	StartTime    time.Time              `json:"start_time"`
	Tags         map[string]interface{} `json:"tags"`
	Events       []SpanEvent            `json:"events"`
	Status       SpanStatus             `json:"status"`
	Component    string                 `json:"component"`
	mu           sync.RWMutex
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name      string                 `json:"name"`
	Time      time.Time              `json:"time"`
	Attributes map[string]interface{} `json:"attributes"`
}

// SpanStatus represents the status of a span
type SpanStatus struct {
	Code        SpanStatusCode `json:"code"`
	Message     string         `json:"message"`
	IsError     bool           `json:"is_error"`
}

// SpanStatusCode represents the status code of a span
type SpanStatusCode string

const (
	SpanStatusOK    SpanStatusCode = "OK"
	SpanStatusError SpanStatusCode = "ERROR"
	SpanStatusAborted SpanStatusCode = "ABORTED"
	SpanStatusCancelled SpanStatusCode = "CANCELLED"
	SpanStatusTimeout SpanStatusCode = "TIMEOUT"
)

// TraceSampler handles trace sampling decisions
type TraceSampler struct {
	config SamplingConfig
	mu     sync.RWMutex
}

// NewTraceManager creates a new trace manager
func NewTraceManager(config TracingConfig) *TraceManager {
	// Create a composite propagator that supports multiple formats
	propagators := []propagation.TextMapPropagator{
		propagation.TraceContext{}, // W3C Trace Context (primary)
		propagation.Baggage{},      // W3C Baggage
	}
	
	return &TraceManager{
		propagator: propagation.NewCompositeTextMapPropagator(propagators...),
		sampler:    TraceSampler{},
		spans:      make(map[string]*SpanInfo),
		config:     config,
		healthStatus: TracingHealthStatus{
			Healthy: true,
		},
	}
}

// ExtractTraceContext extracts trace context from HTTP headers  
func (tm *TraceManager) ExtractTraceContext(r *http.Request) (traceID, spanID string, sampled bool, bg baggage.Baggage) {
	if !tm.config.Enabled {
		return "", "", false, bg
	}
	
	ctx := tm.propagator.Extract(context.Background(), propagation.HeaderCarrier(r.Header))
	
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		traceID = spanCtx.TraceID().String()
		spanID = spanCtx.SpanID().String()
		sampled = spanCtx.IsSampled()
	}
	
	if tm.config.EnableBaggage {
		bg = baggage.FromContext(ctx)
	}
	
	return traceID, spanID, sampled, bg
}

// InjectTraceContext injects trace context into HTTP headers
func (tm *TraceManager) InjectTraceContext(ctx context.Context, headers http.Header) {
	if !tm.config.Enabled {
		return
	}
	
	tm.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// CreateSpan creates a new span
func (tm *TraceManager) CreateSpan(ctx context.Context, operation string, options ...SpanOption) (*SpanInfo, context.Context) {
	if !tm.config.Enabled {
		return nil, ctx
	}
	
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	// Check if we've exceeded the maximum number of spans
	if len(tm.spans) >= tm.config.MaxSpansPerTrace {
		tm.healthStatus.SpansDropped++
		return nil, ctx
	}
	
	// Generate new span
	var parentSpanID string
	traceID := GetTraceID(ctx)
	if traceID == "" {
		traceID = GenerateTraceID()
		ctx = WithTraceID(ctx, traceID)
	} else {
		parentSpanID = GetSpanID(ctx)
	}
	
	spanID := GenerateSpanID()
	ctx = WithSpanID(ctx, spanID)
	
	// Create span info
	span := &SpanInfo{
		TraceID:   traceID,
		SpanID:    spanID,
		ParentID:  parentSpanID,
		Operation: operation,
		StartTime: time.Now(),
		Tags:      make(map[string]interface{}),
		Events:    make([]SpanEvent, 0),
		Status: SpanStatus{
			Code: SpanStatusOK,
		},
		Component: GetComponent(ctx),
	}
	
	// Apply options
	for _, opt := range options {
		opt(span)
	}
	
	// Store span
	tm.spans[spanID] = span
	tm.healthStatus.TotalSpansCreated++
	
	// Start cleanup timer
	go tm.cleanupSpanAfterTimeout(spanID)
	
	return span, ctx
}

// FinishSpan finishes a span
func (tm *TraceManager) FinishSpan(spanID string, err error) {
	if !tm.config.Enabled {
		return
	}
	
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	span, exists := tm.spans[spanID]
	if !exists {
		return
	}
	
	span.mu.Lock()
	defer span.mu.Unlock()
	
	// Set status based on error
	if err != nil {
		span.Status = SpanStatus{
			Code:    SpanStatusError,
			Message: err.Error(),
			IsError: true,
		}
		span.Tags["error"] = true
		span.Tags["error.message"] = err.Error()
	}
	
	// Add finish event
	span.Events = append(span.Events, SpanEvent{
		Name: "span.finish",
		Time: time.Now(),
		Attributes: map[string]interface{}{
			"duration_ms": time.Since(span.StartTime).Milliseconds(),
		},
	})
	
	// Remove from active spans
	delete(tm.spans, spanID)
	tm.healthStatus.TotalSpansFinished++
}

// AddSpanEvent adds an event to a span
func (tm *TraceManager) AddSpanEvent(spanID, eventName string, attributes map[string]interface{}) {
	if !tm.config.Enabled {
		return
	}
	
	tm.mu.RLock()
	span, exists := tm.spans[spanID]
	tm.mu.RUnlock()
	
	if !exists {
		return
	}
	
	span.mu.Lock()
	defer span.mu.Unlock()
	
	event := SpanEvent{
		Name:       eventName,
		Time:       time.Now(),
		Attributes: attributes,
	}
	
	span.Events = append(span.Events, event)
}

// SetSpanTag sets a tag on a span
func (tm *TraceManager) SetSpanTag(spanID, key string, value interface{}) {
	if !tm.config.Enabled {
		return
	}
	
	tm.mu.RLock()
	span, exists := tm.spans[spanID]
	tm.mu.RUnlock()
	
	if !exists {
		return
	}
	
	span.mu.Lock()
	defer span.mu.Unlock()
	
	span.Tags[key] = value
}

// GetActiveSpan returns the active span information
func (tm *TraceManager) GetActiveSpan(spanID string) *SpanInfo {
	if !tm.config.Enabled {
		return nil
	}
	
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	span, exists := tm.spans[spanID]
	if !exists {
		return nil
	}
	
	// Return a copy to avoid race conditions
	span.mu.RLock()
	defer span.mu.RUnlock()
	
	spanCopy := &SpanInfo{
		TraceID:   span.TraceID,
		SpanID:    span.SpanID,
		ParentID:  span.ParentID,
		Operation: span.Operation,
		StartTime: span.StartTime,
		Tags:      make(map[string]interface{}),
		Events:    make([]SpanEvent, len(span.Events)),
		Status:    span.Status,
		Component: span.Component,
	}
	
	// Deep copy tags
	for k, v := range span.Tags {
		spanCopy.Tags[k] = v
	}
	
	// Deep copy events
	copy(spanCopy.Events, span.Events)
	
	return spanCopy
}

// GetAllActiveSpans returns all active spans
func (tm *TraceManager) GetAllActiveSpans() map[string]*SpanInfo {
	if !tm.config.Enabled {
		return nil
	}
	
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	result := make(map[string]*SpanInfo, len(tm.spans))
	
	for spanID := range tm.spans {
		result[spanID] = tm.GetActiveSpan(spanID)
	}
	
	return result
}

// GetTracingHealth returns the health status of the tracing system
func (tm *TraceManager) GetTracingHealth() TracingHealthStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	status := tm.healthStatus
	status.ActiveSpans = len(tm.spans)
	
	// Check if we have too many active spans
	if status.ActiveSpans > tm.config.MaxSpansPerTrace*2 {
		status.Healthy = false
	}
	
	return status
}

// cleanupSpanAfterTimeout removes a span after timeout
func (tm *TraceManager) cleanupSpanAfterTimeout(spanID string) {
	timeout := time.Duration(tm.config.SpanTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute // Default timeout
	}
	
	time.Sleep(timeout)
	
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if span, exists := tm.spans[spanID]; exists {
		// Mark as timed out
		span.mu.Lock()
		span.Status = SpanStatus{
			Code:    SpanStatusTimeout,
			Message: "Span timed out",
			IsError: true,
		}
		span.mu.Unlock()
		
		// Remove from active spans
		delete(tm.spans, spanID)
		tm.healthStatus.SpansDropped++
	}
}

// SpanOption is a function that configures a span
type SpanOption func(*SpanInfo)

// WithSpanTag adds a tag to a span
func WithSpanTag(key string, value interface{}) SpanOption {
	return func(span *SpanInfo) {
		span.Tags[key] = value
	}
}

// WithSpanComponent sets the component for a span
func WithSpanComponent(component string) SpanOption {
	return func(span *SpanInfo) {
		span.Component = component
	}
}

// TraceContextMiddleware provides HTTP middleware for trace context extraction
func (tm *TraceManager) TraceContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !tm.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			
			// Extract trace context from headers
			traceID, spanID, sampled, bag := tm.ExtractTraceContext(r)
			
			// Create request context with trace information
			ctx := r.Context()
			if traceID != "" {
				ctx = WithTraceID(ctx, traceID)
			}
			if spanID != "" {
				ctx = WithSpanID(ctx, spanID)
			}
			
			// Add baggage to context if enabled
			if tm.config.EnableBaggage {
				ctx = baggage.ContextWithBaggage(ctx, bag)
			}
			
			// Create a span for the HTTP request
			operation := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			span, ctx := tm.CreateSpan(ctx, operation,
				WithSpanTag("http.method", r.Method),
				WithSpanTag("http.url", r.URL.String()),
				WithSpanTag("http.scheme", r.URL.Scheme),
				WithSpanTag("http.host", r.Host),
				WithSpanTag("http.user_agent", r.UserAgent()),
				WithSpanTag("sampled", sampled),
				WithSpanComponent("http.server"),
			)
			
			// Create a response wrapper to capture status code
			wrapped := &traceResponseWrapper{
				ResponseWriter: w,
				statusCode:     200,
				tm:             tm,
				span:           span,
			}
			
			// Set trace context in response headers
			tm.InjectTraceContext(ctx, w.Header())
			
			// Process request
			next.ServeHTTP(wrapped, r.WithContext(ctx))
			
			// Finish span
			if span != nil {
				tm.SetSpanTag(span.SpanID, "http.status_code", wrapped.statusCode)
				if wrapped.statusCode >= 400 {
					tm.FinishSpan(span.SpanID, fmt.Errorf("HTTP %d", wrapped.statusCode))
				} else {
					tm.FinishSpan(span.SpanID, nil)
				}
			}
		})
	}
}

// traceResponseWrapper wraps http.ResponseWriter to capture response information
type traceResponseWrapper struct {
	http.ResponseWriter
	statusCode int
	tm         *TraceManager
	span       *SpanInfo
}

func (w *traceResponseWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *traceResponseWrapper) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	if w.span != nil {
		w.tm.SetSpanTag(w.span.SpanID, "http.response_size", n)
	}
	return n, err
}

// ShouldSample determines if a trace should be sampled
func (ts *TraceSampler) ShouldSample(traceID string) bool {
	// Simple hash-based sampling for consistent sampling decisions
	// This ensures that traces with the same ID always get the same sampling decision
	hash := 0
	for _, c := range traceID {
		hash = hash*31 + int(c)
	}
	
	// Convert to positive number between 0 and 1
	normalized := float64(uint32(hash)) / float64(^uint32(0))
	
	ts.mu.RLock()
	rate := ts.config.Rate
	ts.mu.RUnlock()
	
	return normalized < rate
}

// UpdateSamplingConfig updates the sampling configuration
func (ts *TraceSampler) UpdateSamplingConfig(config SamplingConfig) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.config = config
}

// DefaultTracingConfig returns a default tracing configuration
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:            false,
		SamplingRate:       1.0, // Sample all traces by default
		MaxSpansPerTrace:   1000,
		SpanTimeoutSeconds: 300, // 5 minutes
		EnableBaggage:      true,
		MaxBaggageSize:     8192, // 8KB
	}
}

// ParseTraceParentAdvanced parses a W3C Trace Context traceparent header with validation
func ParseTraceParentAdvanced(header string) (traceID, spanID string, sampled bool, valid bool) {
	// Enhanced version of ParseTraceParent with better validation
	if len(header) != 55 {
		return "", "", false, false
	}
	
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return "", "", false, false
	}
	
	// Validate version (must be "00")
	if parts[0] != "00" {
		return "", "", false, false
	}
	
	// Validate trace ID (32 hex chars, not all zeros)
	traceID = parts[1]
	if len(traceID) != 32 {
		return "", "", false, false
	}
	if traceID == "00000000000000000000000000000000" {
		return "", "", false, false
	}
	
	// Validate span ID (16 hex chars, not all zeros)
	spanID = parts[2]
	if len(spanID) != 16 {
		return "", "", false, false
	}
	if spanID == "0000000000000000" {
		return "", "", false, false
	}
	
	// Parse flags
	flags := parts[3]
	if len(flags) != 2 {
		return "", "", false, false
	}
	
	// Check sampled flag (bit 0 of the flags)
	if flagInt, err := strconv.ParseInt(flags, 16, 8); err == nil {
		sampled = (flagInt & 0x01) == 0x01
	}
	
	return traceID, spanID, sampled, true
}

// FormatTraceParentAdvanced formats a W3C Trace Context traceparent header with validation
func FormatTraceParentAdvanced(traceID, spanID string, sampled bool) (string, error) {
	// Validate inputs
	if len(traceID) != 32 {
		return "", fmt.Errorf("invalid trace ID length: expected 32, got %d", len(traceID))
	}
	if len(spanID) != 16 {
		return "", fmt.Errorf("invalid span ID length: expected 16, got %d", len(spanID))
	}
	
	// Validate hex format
	if _, err := strconv.ParseUint(traceID, 16, 64); err != nil {
		return "", fmt.Errorf("invalid trace ID format: %w", err)
	}
	if _, err := strconv.ParseUint(spanID, 16, 64); err != nil {
		return "", fmt.Errorf("invalid span ID format: %w", err)
	}
	
	// Format flags
	flags := "00"
	if sampled {
		flags = "01"
	}
	
	return fmt.Sprintf("00-%s-%s-%s", traceID, spanID, flags), nil
}