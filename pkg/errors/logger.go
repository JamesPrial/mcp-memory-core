package errors

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
)

// Logger provides centralized error logging
type Logger struct {
	logger       *slog.Logger
	metrics      *logging.ErrorMetrics
	auditLogger  *logging.AuditLogger
	useGlobal    bool
}

// NewLogger creates a new error logger using the global logging factory
func NewLogger(component string) *Logger {
	return &Logger{
		logger:    logging.GetGlobalLogger(component),
		metrics:   logging.GetGlobalMetrics(),
		auditLogger: logging.GetGlobalAuditLogger(),
		useGlobal: true,
	}
}

// NewLoggerWithSlog creates a new error logger with a specific slog logger (backward compatibility)
func NewLoggerWithSlog(logger *slog.Logger) *Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &Logger{
		logger:    logger,
		metrics:   logging.GetGlobalMetrics(),
		auditLogger: logging.GetGlobalAuditLogger(),
		useGlobal: false,
	}
}

// LogError logs an error with full context while returning a safe error for clients
func (l *Logger) LogError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}

	// Get component from context or use default
	component := logging.GetComponent(ctx)
	if component == "" {
		component = "unknown"
	}

	// Capture stack trace
	stack := l.captureStack(3) // Skip LogError, captureStack, and the caller

	// Extract context metadata using unified logging utilities
	requestID := logging.GetRequestID(ctx)
	traceID := logging.GetTraceID(ctx)
	userID := logging.GetUserID(ctx)

	// Build base log attributes with unified context
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("component", component),
		slog.String("error_type", fmt.Sprintf("%T", err)),
	}

	// Add context attributes if present
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}

	// Handle AppError specifically
	if appErr, ok := err.(*AppError); ok {
		// Get structured error metadata
		metadata := logging.GetErrorMetadata(string(appErr.Code), operation, component)
		
		// Record error metrics
		if l.metrics != nil {
			l.metrics.RecordError(ctx, string(appErr.Code), operation, component)
		}

		// Add structured metadata
		attrs = append(attrs,
			slog.String("error_code", string(appErr.Code)),
			slog.String("error_message", appErr.Message),
			slog.String("error_category", string(metadata.Category)),
			slog.String("error_severity", string(metadata.Severity)),
			slog.String("error_source", metadata.Source),
		)

		if appErr.Internal != nil {
			attrs = append(attrs,
				slog.String("internal_error", appErr.Internal.Error()),
				slog.String("internal_type", fmt.Sprintf("%T", appErr.Internal)),
			)
		}

		if appErr.Details != nil {
			attrs = append(attrs, slog.Any("error_details", appErr.Details))
		}

		if len(appErr.Stack) > 0 {
			attrs = append(attrs, slog.Any("app_stack", appErr.Stack))
		}

		// Log at appropriate level based on error code
		level := l.getLogLevel(appErr.Code)
		
		// Use context-aware logger if using global factory
		var contextLogger *slog.Logger
		if l.useGlobal {
			// Get logger factory and create context-aware logger
			if factory := l.getFactory(); factory != nil {
				contextLogger = factory.WithContext(ctx, l.logger)
			} else {
				contextLogger = l.logger
			}
		} else {
			contextLogger = l.logger
		}
		
		contextLogger.LogAttrs(ctx, level, "Application error occurred",
			append(attrs, slog.Any("stack_trace", stack))...)

		// Log to audit logger if available and it's a significant error
		if l.auditLogger != nil && (metadata.Severity == logging.SeverityHigh || metadata.Severity == logging.SeverityCritical) {
			event := logging.AuditEvent{
				EventType:   "error",
				Action:      operation,
				Resource:    component,
				Result:      "error",
				Details: map[string]interface{}{
					"error_code": appErr.Code,
					"error_message": appErr.Message,
					"error_category": metadata.Category,
					"error_severity": metadata.Severity,
				},
			}
			l.auditLogger.Log(ctx, event)
		}

		// Return the safe error
		return appErr
	}

	// For non-AppError, wrap it as internal and log
	errorCode := string(ErrCodeInternal)
	metadata := logging.GetErrorMetadata(errorCode, operation, component)
	
	// Record metrics for internal errors
	if l.metrics != nil {
		l.metrics.RecordError(ctx, errorCode, operation, component)
	}

	attrs = append(attrs,
		slog.String("error", err.Error()),
		slog.String("error_code", errorCode),
		slog.String("error_category", string(metadata.Category)),
		slog.String("error_severity", string(metadata.Severity)),
		slog.String("error_source", metadata.Source),
		slog.Any("stack_trace", stack),
	)

	// Use context-aware logger if using global factory
	var contextLogger *slog.Logger
	if l.useGlobal {
		if factory := l.getFactory(); factory != nil {
			contextLogger = factory.WithContext(ctx, l.logger)
		} else {
			contextLogger = l.logger
		}
	} else {
		contextLogger = l.logger
	}

	contextLogger.LogAttrs(ctx, slog.LevelError, "Unexpected error occurred", attrs...)

	// Log to audit logger for critical internal errors
	if l.auditLogger != nil {
		event := logging.AuditEvent{
			EventType:   "error",
			Action:      operation,
			Resource:    component,
			Result:      "error",
			Details: map[string]interface{}{
				"error_message": "Internal error occurred",
				"error_type": fmt.Sprintf("%T", err),
				"error_severity": metadata.Severity,
			},
		}
		l.auditLogger.Log(ctx, event)
	}

	// Return a safe internal error
	return Internal(err)
}

// LogAndWrap logs an error and wraps it with an AppError
func (l *Logger) LogAndWrap(ctx context.Context, err error, code ErrorCode, message, operation string) *AppError {
	if err == nil {
		return nil
	}

	appErr := Wrap(err, code, message).WithStack(operation)
	l.LogError(ctx, appErr, operation)
	return appErr
}

// LogAndWrapf logs an error and wraps it with a formatted message
func (l *Logger) LogAndWrapf(ctx context.Context, err error, code ErrorCode, operation, format string, args ...interface{}) *AppError {
	if err == nil {
		return nil
	}

	appErr := Wrapf(err, code, format, args...).WithStack(operation)
	l.LogError(ctx, appErr, operation)
	return appErr
}

// LogPanic logs a panic and returns an appropriate error
func (l *Logger) LogPanic(ctx context.Context, recovered interface{}, operation string) error {
	component := logging.GetComponent(ctx)
	if component == "" {
		component = "unknown"
	}
	
	stack := l.captureStack(3)
	
	// Extract context metadata
	requestID := logging.GetRequestID(ctx)
	traceID := logging.GetTraceID(ctx)
	userID := logging.GetUserID(ctx)
	
	// Get structured error metadata for panic
	errorCode := string(ErrCodePanic)
	metadata := logging.GetErrorMetadata(errorCode, operation, component)
	
	// Record metrics
	if l.metrics != nil {
		l.metrics.RecordError(ctx, errorCode, operation, component)
	}
	
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("component", component),
		slog.String("error_code", errorCode),
		slog.String("error_category", string(metadata.Category)),
		slog.String("error_severity", string(metadata.Severity)),
		slog.String("error_source", metadata.Source),
		slog.Any("panic_value", recovered),
		slog.Any("stack_trace", stack),
	}

	// Add context attributes
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}

	// Use context-aware logger if using global factory
	var contextLogger *slog.Logger
	if l.useGlobal {
		if factory := l.getFactory(); factory != nil {
			contextLogger = factory.WithContext(ctx, l.logger)
		} else {
			contextLogger = l.logger
		}
	} else {
		contextLogger = l.logger
	}

	contextLogger.LogAttrs(ctx, slog.LevelError, "Panic recovered", attrs...)

	// Log to audit logger for panic (critical event)
	if l.auditLogger != nil {
		event := logging.AuditEvent{
			EventType:   "security",
			Action:      "panic_recovery",
			Resource:    component,
			Result:      "error",
			Details: map[string]interface{}{
				"message": "Panic recovered in application",
				"panic_value": recovered,
				"operation": operation,
				"component": component,
			},
		}
		l.auditLogger.Log(ctx, event)
	}

	// Return a safe error
	return New(ErrCodePanic, "An unexpected error occurred")
}

// getFactory returns the global logging factory if available
func (l *Logger) getFactory() *logging.Factory {
	// This is a stub - in a real implementation, you'd have access to the global factory
	// For now, we return nil and fall back to direct logger usage
	return nil
}

// captureStack captures the current stack trace
func (l *Logger) captureStack(skip int) []string {
	const maxStackSize = 10
	stack := make([]string, 0, maxStackSize)
	
	for i := skip; i < skip+maxStackSize; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		
		// Clean up file path to be relative
		if idx := strings.LastIndex(file, "/mcp-memory-core/"); idx >= 0 {
			file = file[idx+len("/mcp-memory-core/"):]
		}
		
		stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
	}
	
	return stack
}


// getLogLevel determines the appropriate log level for an error code
func (l *Logger) getLogLevel(code ErrorCode) slog.Level {
	switch {
	case strings.HasPrefix(string(code), "VALIDATION_"):
		return slog.LevelWarn
	case strings.HasPrefix(string(code), "AUTH_"):
		return slog.LevelWarn
	case code == ErrCodeEntityNotFound || code == ErrCodeRelationNotFound:
		return slog.LevelInfo
	case strings.HasPrefix(string(code), "STORAGE_"):
		return slog.LevelError
	case strings.HasPrefix(string(code), "INTERNAL") || code == ErrCodePanic:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// Default logger instance with thread-safe initialization
var (
	defaultLogger *Logger
	defaultLoggerOnce sync.Once
	defaultLoggerMu   sync.RWMutex
)

// getDefaultLogger returns the default logger, initializing it if necessary
func getDefaultLogger() *Logger {
	defaultLoggerOnce.Do(func() {
		defaultLogger = NewLogger("errors")
	})
	defaultLoggerMu.RLock()
	defer defaultLoggerMu.RUnlock()
	return defaultLogger
}

// SetDefaultLogger sets the default error logger in a thread-safe manner
func SetDefaultLogger(logger *slog.Logger) {
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	defaultLogger = NewLoggerWithSlog(logger)
}

// SetDefaultLoggerComponent sets the default error logger for a specific component
func SetDefaultLoggerComponent(component string) {
	defaultLoggerMu.Lock()
	defer defaultLoggerMu.Unlock()
	defaultLogger = NewLogger(component)
}

// LogError logs an error using the default logger
func LogError(ctx context.Context, err error, operation string) error {
	return getDefaultLogger().LogError(ctx, err, operation)
}

// LogAndWrap logs and wraps an error using the default logger
func LogAndWrap(ctx context.Context, err error, code ErrorCode, message, operation string) *AppError {
	return getDefaultLogger().LogAndWrap(ctx, err, code, message, operation)
}

// LogAndWrapf logs and wraps an error with formatted message using the default logger
func LogAndWrapf(ctx context.Context, err error, code ErrorCode, operation, format string, args ...interface{}) *AppError {
	return getDefaultLogger().LogAndWrapf(ctx, err, code, operation, format, args...)
}

// LogPanic logs a panic using the default logger
func LogPanic(ctx context.Context, recovered interface{}, operation string) error {
	return getDefaultLogger().LogPanic(ctx, recovered, operation)
}