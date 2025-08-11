package errors

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// Logger provides centralized error logging
type Logger struct {
	logger *slog.Logger
}

// NewLogger creates a new error logger
func NewLogger(logger *slog.Logger) *Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &Logger{logger: logger}
}

// LogError logs an error with full context while returning a safe error for clients
func (l *Logger) LogError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}

	// Capture stack trace
	stack := l.captureStack(3) // Skip LogError, captureStack, and the caller

	// Extract request ID from context if available
	requestID := l.extractRequestID(ctx)

	// Build log attributes
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.String("error_type", fmt.Sprintf("%T", err)),
	}

	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	// Handle AppError specifically
	if appErr, ok := err.(*AppError); ok {
		attrs = append(attrs,
			slog.String("error_code", string(appErr.Code)),
			slog.String("error_message", appErr.Message),
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
		l.logger.LogAttrs(ctx, level, "Application error occurred",
			append(attrs, slog.Any("stack_trace", stack))...)

		// Return the safe error
		return appErr
	}

	// For non-AppError, wrap it as internal and log
	attrs = append(attrs,
		slog.String("error", err.Error()),
		slog.Any("stack_trace", stack),
	)

	l.logger.LogAttrs(ctx, slog.LevelError, "Unexpected error occurred", attrs...)

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
	stack := l.captureStack(3)
	
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Any("panic_value", recovered),
		slog.Any("stack_trace", stack),
	}

	if requestID := l.extractRequestID(ctx); requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	l.logger.LogAttrs(ctx, slog.LevelError, "Panic recovered", attrs...)

	// Return a safe error
	return New(ErrCodePanic, "An unexpected error occurred")
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

// extractRequestID extracts request ID from context
func (l *Logger) extractRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	
	// Try common keys for request ID
	keys := []string{"request_id", "requestId", "req_id", "trace_id"}
	for _, key := range keys {
		if val := ctx.Value(key); val != nil {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	
	return ""
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

// Default logger instance
var defaultLogger = NewLogger(nil)

// SetDefaultLogger sets the default error logger
func SetDefaultLogger(logger *slog.Logger) {
	defaultLogger = NewLogger(logger)
}

// LogError logs an error using the default logger
func LogError(ctx context.Context, err error, operation string) error {
	return defaultLogger.LogError(ctx, err, operation)
}

// LogAndWrap logs and wraps an error using the default logger
func LogAndWrap(ctx context.Context, err error, code ErrorCode, message, operation string) *AppError {
	return defaultLogger.LogAndWrap(ctx, err, code, message, operation)
}

// LogAndWrapf logs and wraps an error with formatted message using the default logger
func LogAndWrapf(ctx context.Context, err error, code ErrorCode, operation, format string, args ...interface{}) *AppError {
	return defaultLogger.LogAndWrapf(ctx, err, code, operation, format, args...)
}

// LogPanic logs a panic using the default logger
func LogPanic(ctx context.Context, recovered interface{}, operation string) error {
	return defaultLogger.LogPanic(ctx, recovered, operation)
}