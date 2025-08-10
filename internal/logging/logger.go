package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// RequestIDKey is the context key for request correlation IDs
	RequestIDKey contextKey = "request_id"
	// MethodKey is the context key for the current method being executed
	MethodKey contextKey = "method"
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // "json" or "text"
}

// NewLogger creates a new logger with the specified configuration
func NewLogger(cfg *LogConfig) *Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize source location format
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					// Simplify the source path to just show filename
					if idx := strings.LastIndex(source.File, "/"); idx >= 0 {
						a.Value = slog.StringValue(fmt.Sprintf("%s:%d", source.File[idx+1:], source.Line))
					} else {
						a.Value = slog.StringValue(fmt.Sprintf("%s:%d", source.File, source.Line))
					}
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithContext returns a logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l.Logger

	// Add request ID if present
	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		logger = logger.With("request_id", requestID)
	}

	// Add method if present
	if method := ctx.Value(MethodKey); method != nil {
		logger = logger.With("method", method)
	}

	return &Logger{Logger: logger}
}

// WithRequestID adds a request ID to the logger
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With("request_id", requestID),
	}
}

// WithMethod adds a method name to the logger
func (l *Logger) WithMethod(method string) *Logger {
	return &Logger{
		Logger: l.Logger.With("method", method),
	}
}

// WithError adds an error to the logger
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return &Logger{
		Logger: l.Logger.With("error", err.Error()),
	}
}

// WithField adds a custom field to the logger
func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{
		Logger: l.Logger.With(key, value),
	}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// LogRequest logs an incoming request
func (l *Logger) LogRequest(ctx context.Context, method string, params map[string]interface{}) {
	l.WithContext(ctx).
		WithMethod(method).
		WithField("params", params).
		Info("Request received")
}

// LogResponse logs an outgoing response
func (l *Logger) LogResponse(ctx context.Context, method string, duration float64, err error) {
	logger := l.WithContext(ctx).
		WithMethod(method).
		WithField("duration_ms", duration)

	if err != nil {
		logger.WithError(err).Error("Request failed")
	} else {
		logger.Info("Request completed")
	}
}

// LogDatabaseOperation logs database operations
func (l *Logger) LogDatabaseOperation(operation string, query string, duration float64, err error) {
	logger := l.WithFields(map[string]any{
		"operation":   operation,
		"query":       query,
		"duration_ms": duration,
	})

	if err != nil {
		logger.WithError(err).Error("Database operation failed")
	} else {
		logger.Debug("Database operation completed")
	}
}

// DefaultLogger creates a default logger with INFO level and text format
func DefaultLogger() *Logger {
	return NewLogger(&LogConfig{
		Level:  "info",
		Format: "text",
	})
}