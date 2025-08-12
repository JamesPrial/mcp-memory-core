package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Factory creates and manages loggers for different components
type Factory struct {
	config  *Config
	loggers map[string]*slog.Logger
	mu      sync.RWMutex
	
	// Shared resources
	handler         slog.Handler
	auditLogger     *AuditLogger
	sampler         *Sampler
	masker          *Masker
	metricsCollector *MetricsCollector
}

// NewFactory creates a new logger factory
func NewFactory(config *Config) (*Factory, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logging config: %w", err)
	}
	
	f := &Factory{
		config:  config,
		loggers: make(map[string]*slog.Logger),
	}
	
	// Initialize sampler BEFORE handler (needed for handler wrapping)
	if config.Sampling.Enabled {
		f.sampler = NewSampler(config.Sampling)
	}
	
	// Initialize masker BEFORE handler (needed for ReplaceAttr)
	if config.Masking.Enabled {
		f.masker = NewMasker(config.Masking)
	}
	
	// Initialize base handler (uses masker and sampler)
	if err := f.initializeHandler(); err != nil {
		return nil, fmt.Errorf("failed to initialize handler: %w", err)
	}
	
	// Initialize audit logger if enabled
	if config.EnableAudit {
		auditLogger, err := NewAuditLogger(config.AuditFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize audit logger: %w", err)
		}
		f.auditLogger = auditLogger
	}
	
	// Initialize metrics collector if enabled
	if config.Metrics.Enabled {
		f.metricsCollector = NewMetricsCollector(config.Metrics)
		// Set metrics for audit logger if both are enabled
		if f.auditLogger != nil {
			f.auditLogger.SetMetrics(f.metricsCollector)
		}
	}
	
	return f, nil
}

// initializeHandler creates the base slog handler
func (f *Factory) initializeHandler() error {
	var writer io.Writer
	
	switch f.config.Output {
	case LogOutputStdout:
		writer = os.Stdout
	case LogOutputStderr:
		writer = os.Stderr
	case LogOutputFile:
		file, err := os.OpenFile(f.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		writer = file
	default:
		writer = os.Stdout
	}
	
	// Configure handler options
	opts := &slog.HandlerOptions{
		Level:     f.slogLevel(f.config.Level),
		AddSource: f.config.EnableCaller,
	}
	
	// Add custom replacer for masking and enrichment
	if f.masker != nil || f.config.EnableRequestID {
		opts.ReplaceAttr = f.replaceAttr
	}
	
	// Create base handler
	switch f.config.Format {
	case LogFormatJSON:
		f.handler = slog.NewJSONHandler(writer, opts)
	case LogFormatText:
		f.handler = slog.NewTextHandler(writer, opts)
	default:
		f.handler = slog.NewJSONHandler(writer, opts)
	}
	
	// Wrap with sampling if enabled
	if f.sampler != nil {
		f.handler = NewSamplingHandler(f.handler, f.sampler)
	}
	
	// Wrap with async handler if enabled
	if f.config.AsyncLogging {
		f.handler = NewAsyncHandler(f.handler, f.config.BufferSize)
	}
	
	// Wrap with OTLP export if enabled (placeholder implementation)
	if f.config.OTLP.Enabled {
		// TODO: Implement OTLP handler
		// For now, just use the existing handler
		// otlpHandler, err := NewOTLPHandler(f.handler, f.config.OTLP)
		// if err != nil {
		//     return fmt.Errorf("failed to create OTLP handler: %w", err)
		// }
		// f.handler = otlpHandler
	}
	
	return nil
}

// GetLogger returns a logger for a specific component
func (f *Factory) GetLogger(component string) *slog.Logger {
	f.mu.RLock()
	if logger, exists := f.loggers[component]; exists {
		f.mu.RUnlock()
		return logger
	}
	f.mu.RUnlock()
	
	// Create new logger for component
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Double-check after acquiring write lock
	if logger, exists := f.loggers[component]; exists {
		return logger
	}
	
	// Create component-specific handler with appropriate level
	level := f.config.GetLevelForComponent(component)
	handler := f.handler
	
	// If component has a different level, wrap the handler
	if level != f.config.Level {
		handler = NewLevelHandler(handler, f.slogLevel(level))
	}
	
	// Create logger with component context
	logger := slog.New(handler).With(
		slog.String("component", component),
	)
	
	f.loggers[component] = logger
	return logger
}

// GetAuditLogger returns the audit logger
func (f *Factory) GetAuditLogger() *AuditLogger {
	return f.auditLogger
}

// GetMetricsCollector returns the metrics collector
func (f *Factory) GetMetricsCollector() *MetricsCollector {
	return f.metricsCollector
}

// GetMasker returns the data masker
func (f *Factory) GetMasker() *Masker {
	return f.masker
}

// GetSampler returns the log sampler
func (f *Factory) GetSampler() *Sampler {
	return f.sampler
}

// WithContext creates a logger with request context
func (f *Factory) WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = f.GetLogger("default")
	}
	
	// Extract context values
	attrs := []slog.Attr{}
	
	// Add request ID if present
	if reqID := GetRequestID(ctx); reqID != "" {
		attrs = append(attrs, slog.String("request_id", reqID))
	}
	
	// Add trace ID if present
	if traceID := GetTraceID(ctx); traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	
	// Add user ID if present
	if userID := GetUserID(ctx); userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}
	
	// Add operation if present
	if operation := GetOperation(ctx); operation != "" {
		attrs = append(attrs, slog.String("operation", operation))
	}
	
	if len(attrs) > 0 {
		args := make([]any, 0, len(attrs)*2)
		for _, attr := range attrs {
			args = append(args, attr.Key, attr.Value)
		}
		return logger.With(args...)
	}
	
	return logger
}

// replaceAttr handles attribute replacement for masking and enrichment
func (f *Factory) replaceAttr(groups []string, a slog.Attr) slog.Attr {
	// Apply masking if enabled
	if f.masker != nil {
		a = f.masker.MaskAttr(groups, a)
	}
	
	// Add timestamp format standardization
	if a.Key == slog.TimeKey {
		if t, ok := a.Value.Any().(string); ok {
			a.Value = slog.StringValue(t)
		}
	}
	
	return a
}

// slogLevel converts our LogLevel to slog.Level
func (f *Factory) slogLevel(level LogLevel) slog.Level {
	switch level {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// UpdateLevel dynamically updates the log level for a component
func (f *Factory) UpdateLevel(component string, level LogLevel) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if f.config.ComponentLevels == nil {
		f.config.ComponentLevels = make(map[string]LogLevel)
	}
	f.config.ComponentLevels[component] = level
	
	// Remove cached logger to force recreation with new level
	delete(f.loggers, component)
}

// Close closes all resources
func (f *Factory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	var errs []error
	
	// Close audit logger
	if f.auditLogger != nil {
		if err := f.auditLogger.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close audit logger: %w", err))
		}
	}
	
	// Close metrics collector
	if f.metricsCollector != nil {
		if err := f.metricsCollector.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close metrics collector: %w", err))
		}
	}
	
	// Close async handler if exists
	if asyncHandler, ok := f.handler.(*AsyncHandler); ok {
		if err := asyncHandler.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close async handler: %w", err))
		}
	}
	
	// Close OTLP handler if exists (placeholder for when implemented)
	// if otlpHandler, ok := f.handler.(*OTLPHandler); ok {
	//     if err := otlpHandler.Close(); err != nil {
	//         errs = append(errs, fmt.Errorf("failed to close OTLP handler: %w", err))
	//     }
	// }
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing logger factory: %v", errs)
	}
	
	return nil
}

// Global factory instance
var (
	globalFactory *Factory
	globalMu      sync.RWMutex
)

// Initialize sets up the global logger factory
func Initialize(config *Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()
	
	if globalFactory != nil {
		if err := globalFactory.Close(); err != nil {
			return fmt.Errorf("failed to close existing factory: %w", err)
		}
	}
	
	factory, err := NewFactory(config)
	if err != nil {
		return err
	}
	
	globalFactory = factory
	return nil
}

// GetGlobalLogger returns a logger from the global factory
func GetGlobalLogger(component string) *slog.Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalFactory == nil {
		// Return default logger if not initialized
		return slog.Default()
	}
	
	return globalFactory.GetLogger(component)
}

// GetGlobalAuditLogger returns the global audit logger
func GetGlobalAuditLogger() *AuditLogger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalFactory == nil {
		return nil
	}
	
	return globalFactory.GetAuditLogger()
}

// GetGlobalMetricsCollector returns the global metrics collector
func GetGlobalMetricsCollector() *MetricsCollector {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalFactory == nil {
		return nil
	}
	
	return globalFactory.GetMetricsCollector()
}

// GetGlobalMasker returns the global data masker
func GetGlobalMasker() *Masker {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalFactory == nil {
		return nil
	}
	
	return globalFactory.GetMasker()
}

// GetGlobalSampler returns the global log sampler
func GetGlobalSampler() *Sampler {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalFactory == nil {
		return nil
	}
	
	return globalFactory.GetSampler()
}

// Shutdown gracefully shuts down the global logging factory
func Shutdown() error {
	globalMu.Lock()
	defer globalMu.Unlock()
	
	if globalFactory == nil {
		return nil
	}
	
	err := globalFactory.Close()
	globalFactory = nil
	return err
}

// UpdateGlobalLevel dynamically updates the log level for a component
func UpdateGlobalLevel(component string, level LogLevel) {
	globalMu.RLock()
	defer globalMu.RUnlock()
	
	if globalFactory == nil {
		return
	}
	
	globalFactory.UpdateLevel(component, level)
}