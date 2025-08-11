package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

// RequestInterceptor provides request lifecycle logging capabilities
type RequestInterceptor struct {
	logger *slog.Logger
}

// NewRequestInterceptor creates a new request interceptor with the specified logger
func NewRequestInterceptor(logger *slog.Logger) *RequestInterceptor {
	return &RequestInterceptor{
		logger: logger,
	}
}

// InterceptRequest wraps a function with request lifecycle logging
func (r *RequestInterceptor) InterceptRequest(ctx context.Context, operation string, fn func(context.Context) error) error {
	// Generate request context with timing
	ctx = NewRequestContext(ctx, operation)
	startTime := time.Now()
	
	// Get request ID for correlation
	requestID := GetRequestID(ctx)
	
	// Log request start
	r.logger.InfoContext(ctx, "Request started",
		slog.String("operation", operation),
		slog.String("request_id", requestID),
		slog.Time("start_time", startTime),
	)
	
	// Handle panics
	defer func() {
		if recovered := recover(); recovered != nil {
			duration := time.Since(startTime)
			
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			stackTrace := string(buf[:n])
			
			r.logger.ErrorContext(ctx, "Request panicked",
				slog.String("operation", operation),
				slog.String("request_id", requestID),
				slog.Duration("duration", duration),
				slog.Any("panic", recovered),
				slog.String("stack_trace", stackTrace),
			)
			
			// Re-panic to maintain expected behavior
			panic(recovered)
		}
	}()
	
	// Execute the function
	err := fn(ctx)
	duration := time.Since(startTime)
	
	// Log request completion
	if err != nil {
		r.logger.ErrorContext(ctx, "Request completed with error",
			slog.String("operation", operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		r.logger.InfoContext(ctx, "Request completed successfully",
			slog.String("operation", operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
		)
	}
	
	return err
}

// InterceptWithResponse wraps a function that returns a response with request lifecycle logging
func (r *RequestInterceptor) InterceptWithResponse(ctx context.Context, operation string, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	// Generate request context with timing
	ctx = NewRequestContext(ctx, operation)
	startTime := time.Now()
	
	// Get request ID for correlation
	requestID := GetRequestID(ctx)
	
	// Log request start
	r.logger.InfoContext(ctx, "Request started",
		slog.String("operation", operation),
		slog.String("request_id", requestID),
		slog.Time("start_time", startTime),
	)
	
	// Handle panics
	defer func() {
		if recovered := recover(); recovered != nil {
			duration := time.Since(startTime)
			
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			stackTrace := string(buf[:n])
			
			r.logger.ErrorContext(ctx, "Request panicked",
				slog.String("operation", operation),
				slog.String("request_id", requestID),
				slog.Duration("duration", duration),
				slog.Any("panic", recovered),
				slog.String("stack_trace", stackTrace),
			)
			
			// Re-panic to maintain expected behavior
			panic(recovered)
		}
	}()
	
	// Execute the function
	response, err := fn(ctx)
	duration := time.Since(startTime)
	
	// Log request completion
	if err != nil {
		r.logger.ErrorContext(ctx, "Request completed with error",
			slog.String("operation", operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		r.logger.InfoContext(ctx, "Request completed successfully",
			slog.String("operation", operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
		)
	}
	
	return response, err
}

// HTTPMiddleware returns an HTTP middleware that logs request lifecycle
func (r *RequestInterceptor) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		
		// Create request context with operation
		ctx := req.Context()
		operation := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
		ctx = NewRequestContext(ctx, operation)
		
		// Update request with new context
		req = req.WithContext(ctx)
		
		// Get request ID for correlation
		requestID := GetRequestID(ctx)
		
		// Log request start
		r.logger.InfoContext(ctx, "HTTP request started",
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.String("remote_addr", req.RemoteAddr),
			slog.String("user_agent", req.UserAgent()),
			slog.String("request_id", requestID),
			slog.Time("start_time", startTime),
		)
		
		// Create a response writer wrapper to capture status code
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}
		
		// Handle panics
		defer func() {
			if recovered := recover(); recovered != nil {
				duration := time.Since(startTime)
				
				// Get stack trace
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])
				
				r.logger.ErrorContext(ctx, "HTTP request panicked",
					slog.String("method", req.Method),
					slog.String("path", req.URL.Path),
					slog.String("request_id", requestID),
					slog.Duration("duration", duration),
					slog.Any("panic", recovered),
					slog.String("stack_trace", stackTrace),
				)
				
				// Send 500 response if not already sent
				if !wrappedWriter.headerWritten {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}
		}()
		
		// Call the next handler
		next.ServeHTTP(wrappedWriter, req)
		
		// Calculate duration
		duration := time.Since(startTime)
		
		// Log request completion
		statusCode := wrappedWriter.statusCode
		if statusCode >= 400 {
			r.logger.WarnContext(ctx, "HTTP request completed with error",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.String("request_id", requestID),
				slog.Int("status_code", statusCode),
				slog.Duration("duration", duration),
			)
		} else {
			r.logger.InfoContext(ctx, "HTTP request completed successfully",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.String("request_id", requestID),
				slog.Int("status_code", statusCode),
				slog.Duration("duration", duration),
			)
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode    int
	headerWritten bool
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.headerWritten = true
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.headerWritten = true
	}
	return rw.ResponseWriter.Write(b)
}

// OperationTimer helps track operation latencies
type OperationTimer struct {
	logger    *slog.Logger
	operation string
	startTime time.Time
	ctx       context.Context
}

// StartTimer creates a new operation timer
func StartTimer(ctx context.Context, logger *slog.Logger, operation string) *OperationTimer {
	startTime := time.Now()
	
	// Preserve existing context but ensure we have a request ID
	if GetRequestID(ctx) == "" {
		ctx = NewRequestContext(ctx, operation)
	}
	
	timer := &OperationTimer{
		logger:    logger,
		operation: operation,
		startTime: startTime,
		ctx:       ctx,
	}
	
	requestID := GetRequestID(ctx)
	logger.InfoContext(ctx, "Operation started",
		slog.String("operation", operation),
		slog.String("request_id", requestID),
		slog.Time("start_time", startTime),
	)
	
	return timer
}

// End completes the timer and logs the duration
func (t *OperationTimer) End() time.Duration {
	duration := time.Since(t.startTime)
	
	requestID := GetRequestID(t.ctx)
	t.logger.InfoContext(t.ctx, "Operation completed",
		slog.String("operation", t.operation),
		slog.String("request_id", requestID),
		slog.Duration("duration", duration),
	)
	
	return duration
}

// EndWithError completes the timer and logs the duration with an error
func (t *OperationTimer) EndWithError(err error) time.Duration {
	duration := time.Since(t.startTime)
	requestID := GetRequestID(t.ctx)
	
	if err != nil {
		t.logger.ErrorContext(t.ctx, "Operation failed",
			slog.String("operation", t.operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		t.logger.InfoContext(t.ctx, "Operation completed",
			slog.String("operation", t.operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
		)
	}
	
	return duration
}

// LogLatency is a helper function to log operation latencies
func LogLatency(ctx context.Context, logger *slog.Logger, operation string, duration time.Duration, err error) {
	requestID := GetRequestID(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Operation completed with error",
			slog.String("operation", operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		logger.InfoContext(ctx, "Operation completed",
			slog.String("operation", operation),
			slog.String("request_id", requestID),
			slog.Duration("duration", duration),
		)
	}
}