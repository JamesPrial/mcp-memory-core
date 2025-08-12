package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewFactory(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errMsg    string
	}{
		{
			name:      "nil config uses default",
			config:    nil,
			expectErr: false,
		},
		{
			name:      "valid default config",
			config:    DefaultConfig(),
			expectErr: false,
		},
		{
			name:      "valid development config",
			config:    DevelopmentConfig(),
			expectErr: false,
		},
		{
			name:      "valid production config",
			config:    ProductionConfig(),
			expectErr: false,
		},
		{
			name: "invalid config",
			config: &Config{
				Level:  LogLevel("invalid"),
				Format: LogFormatJSON,
				Output: LogOutputStdout,
			},
			expectErr: true,
			errMsg:    "invalid logging config",
		},
		{
			name: "file output with valid path",
			config: &Config{
				Level:    LogLevelInfo,
				Format:   LogFormatJSON,
				Output:   LogOutputFile,
				FilePath: filepath.Join(os.TempDir(), "test.log"),
			},
			expectErr: false,
		},
		{
			name: "audit logging enabled",
			config: &Config{
				Level:         LogLevelInfo,
				Format:        LogFormatJSON,
				Output:        LogOutputStdout,
				EnableAudit:   true,
				AuditFilePath: filepath.Join(os.TempDir(), "audit_test.log"),
			},
			expectErr: false,
		},
		{
			name: "sampling enabled",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				Sampling: SamplingConfig{
					Enabled:      true,
					Rate:         0.5,
					BurstSize:    100,
					AlwaysErrors: true,
				},
			},
			expectErr: false,
		},
		{
			name: "masking enabled",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				Masking: MaskingConfig{
					Enabled:     true,
					Fields:      []string{"password"},
					MaskEmails:  true,
					MaskAPIKeys: true,
				},
			},
			expectErr: false,
		},
		{
			name: "metrics enabled",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				Metrics: MetricsConfig{
					Enabled:    true,
					ListenAddr: ":9090",
					Path:       "/metrics",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFactory(tt.config)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected factory creation to fail but it succeeded")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Expected factory creation to succeed but got error: %v", err)
				return
			}

			if factory == nil {
				t.Error("Expected factory to be created but got nil")
				return
			}

			// Verify factory components are initialized correctly
			if factory.config == nil {
				t.Error("Expected factory config to be set")
			}

			if factory.loggers == nil {
				t.Error("Expected factory loggers map to be initialized")
			}

			if factory.handler == nil {
				t.Error("Expected factory handler to be initialized")
			}

			// Test configuration-specific components
			config := factory.config
			if config.EnableAudit && factory.auditLogger == nil {
				t.Error("Expected audit logger to be initialized when audit is enabled")
			}

			if config.Sampling.Enabled && factory.sampler == nil {
				t.Error("Expected sampler to be initialized when sampling is enabled")
			}

			if config.Masking.Enabled && factory.masker == nil {
				t.Error("Expected masker to be initialized when masking is enabled")
			}

			if config.Metrics.Enabled && factory.metricsCollector == nil {
				t.Error("Expected metrics collector to be initialized when metrics are enabled")
			}

			// Clean up
			factory.Close()
		})
	}
}

func TestFactory_GetLogger(t *testing.T) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	tests := []struct {
		name      string
		component string
	}{
		{"default component", "default"},
		{"database component", "database"},
		{"auth component", "auth"},
		{"empty component", ""},
		{"special characters", "test-component_123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := factory.GetLogger(tt.component)

			if logger == nil {
				t.Error("Expected logger to be returned but got nil")
				return
			}

			// Verify logger works
			logger.Info("Test message")

			// Get the same logger again - should be cached
			logger2 := factory.GetLogger(tt.component)
			if logger != logger2 {
				t.Error("Expected same logger instance to be returned from cache")
			}
		})
	}
}

func TestFactory_GetLoggerWithComponentLevels(t *testing.T) {
	config := &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		ComponentLevels: map[string]LogLevel{
			"database": LogLevelError,
			"auth":     LogLevelDebug,
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	// Test component with specific level
	dbLogger := factory.GetLogger("database")
	if dbLogger == nil {
		t.Error("Expected database logger to be created")
	}

	authLogger := factory.GetLogger("auth")
	if authLogger == nil {
		t.Error("Expected auth logger to be created")
	}

	// Test component with default level
	defaultLogger := factory.GetLogger("unknown")
	if defaultLogger == nil {
		t.Error("Expected default logger to be created")
	}
}

func TestFactory_WithContext(t *testing.T) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("test")
	ctx := CreateTestContext()

	// Test with context
	contextLogger := factory.WithContext(ctx, logger)
	if contextLogger == nil {
		t.Error("Expected context logger to be returned")
	}

	// Test with nil logger - should use default
	contextLogger2 := factory.WithContext(ctx, nil)
	if contextLogger2 == nil {
		t.Error("Expected default context logger to be returned")
	}

	// Test with empty context
	emptyCtx := context.Background()
	contextLogger3 := factory.WithContext(emptyCtx, logger)
	if contextLogger3 == nil {
		t.Error("Expected logger to be returned even with empty context")
	}
}

func TestFactory_GetComponents(t *testing.T) {
	config := &Config{
		Level:       LogLevelInfo,
		Format:      LogFormatJSON,
		Output:      LogOutputStdout,
		EnableAudit: true,
		Sampling: SamplingConfig{
			Enabled: true,
			Rate:    0.5,
		},
		Masking: MaskingConfig{
			Enabled: true,
		},
		Metrics: MetricsConfig{
			Enabled: true,
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	// Test audit logger
	auditLogger := factory.GetAuditLogger()
	if auditLogger == nil {
		t.Error("Expected audit logger to be available")
	}

	// Test metrics collector
	metricsCollector := factory.GetMetricsCollector()
	if metricsCollector == nil {
		t.Error("Expected metrics collector to be available")
	}

	// Test masker
	masker := factory.GetMasker()
	if masker == nil {
		t.Error("Expected masker to be available")
	}

	// Test sampler
	sampler := factory.GetSampler()
	if sampler == nil {
		t.Error("Expected sampler to be available")
	}
}

func TestFactory_UpdateLevel(t *testing.T) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	component := "test-component"

	// Create a logger first
	logger1 := factory.GetLogger(component)
	if logger1 == nil {
		t.Fatal("Expected logger to be created")
	}

	// Update the level
	factory.UpdateLevel(component, LogLevelError)

	// Get the logger again - should be a new instance with the updated level
	logger2 := factory.GetLogger(component)
	if logger2 == nil {
		t.Fatal("Expected logger to be created after level update")
	}

	// Verify the level was updated in config
	if factory.config.ComponentLevels[component] != LogLevelError {
		t.Errorf("Expected component level to be updated to error, got %s", factory.config.ComponentLevels[component])
	}
}

func TestFactory_Close(t *testing.T) {
	config := &Config{
		Level:         LogLevelInfo,
		Format:        LogFormatJSON,
		Output:        LogOutputStdout,
		AsyncLogging:  true,
		EnableAudit:   true,
		AuditFilePath: filepath.Join(os.TempDir(), "test_close_audit.log"),
		Metrics: MetricsConfig{
			Enabled: true,
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	// Use the factory to create some loggers and components
	factory.GetLogger("test1")
	factory.GetLogger("test2")

	// Close should not error
	err = factory.Close()
	if err != nil {
		t.Errorf("Expected Close() to succeed but got error: %v", err)
	}

	// Second close should also not error
	err = factory.Close()
	if err != nil {
		t.Errorf("Expected second Close() to succeed but got error: %v", err)
	}
}

func TestGlobalFunctions(t *testing.T) {
	// Test initialization
	config := DefaultConfig()
	err := Initialize(config)
	if err != nil {
		t.Errorf("Expected Initialize to succeed but got error: %v", err)
	}

	// Test global logger
	logger := GetGlobalLogger("test")
	if logger == nil {
		t.Error("Expected global logger to be returned")
	}

	// Test that we can use the logger
	logger.Info("Test message from global logger")

	// Test global components
	auditLogger := GetGlobalAuditLogger()
	if auditLogger != nil {
		// Audit logger should be nil since it's not enabled in default config
		t.Error("Expected global audit logger to be nil when not configured")
	}

	metricsCollector := GetGlobalMetricsCollector()
	if metricsCollector == nil {
		t.Error("Expected global metrics collector to be available")
	}

	masker := GetGlobalMasker()
	if masker == nil {
		t.Error("Expected global masker to be available")
	}

	sampler := GetGlobalSampler()
	if sampler != nil {
		// Sampler should be nil since it's not enabled in default config
		t.Error("Expected global sampler to be nil when not configured")
	}

	// Test re-initialization
	newConfig := DevelopmentConfig()
	err = Initialize(newConfig)
	if err != nil {
		t.Errorf("Expected re-initialization to succeed but got error: %v", err)
	}

	// Test global logger after re-init
	newLogger := GetGlobalLogger("test")
	if newLogger == nil {
		t.Error("Expected new global logger to be returned after re-init")
	}
}

func TestGlobalFunctionsWithoutInit(t *testing.T) {
	// Reset global factory to test uninitialized state
	globalMu.Lock()
	oldFactory := globalFactory
	globalFactory = nil
	globalMu.Unlock()

	// Restore after test
	defer func() {
		globalMu.Lock()
		globalFactory = oldFactory
		globalMu.Unlock()
	}()

	// Test global logger without initialization - should return default slog logger
	logger := GetGlobalLogger("test")
	if logger == nil {
		t.Error("Expected default logger to be returned when not initialized")
	}

	// Test global components without initialization - should return nil
	auditLogger := GetGlobalAuditLogger()
	if auditLogger != nil {
		t.Error("Expected nil audit logger when not initialized")
	}

	metricsCollector := GetGlobalMetricsCollector()
	if metricsCollector != nil {
		t.Error("Expected nil metrics collector when not initialized")
	}

	masker := GetGlobalMasker()
	if masker != nil {
		t.Error("Expected nil masker when not initialized")
	}

	sampler := GetGlobalSampler()
	if sampler != nil {
		t.Error("Expected nil sampler when not initialized")
	}
}

func TestFactory_ConcurrentAccess(t *testing.T) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	// Test concurrent access to GetLogger
	const numGoroutines = 10
	const numIterations = 100

	done := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < numIterations; j++ {
				component := fmt.Sprintf("component-%d", id)
				logger := factory.GetLogger(component)
				if logger == nil {
					t.Errorf("Expected logger for component %s", component)
					return
				}

				// Use the logger
				logger.Info("Concurrent test", slog.Int("goroutine", id), slog.Int("iteration", j))

				// Also test UpdateLevel concurrently
				if j%10 == 0 {
					factory.UpdateLevel(component, LogLevelDebug)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestFactory_OutputFormats(t *testing.T) {
	tests := []struct {
		name   string
		format LogFormat
		output LogOutput
	}{
		{"JSON stdout", LogFormatJSON, LogOutputStdout},
		{"JSON stderr", LogFormatJSON, LogOutputStderr},
		{"Text stdout", LogFormatText, LogOutputStdout},
		{"Text stderr", LogFormatText, LogOutputStderr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Level:  LogLevelInfo,
				Format: tt.format,
				Output: tt.output,
			}

			factory, err := NewFactory(config)
			if err != nil {
				t.Errorf("Failed to create factory with %s format and %s output: %v", tt.format, tt.output, err)
				return
			}
			defer factory.Close()

			logger := factory.GetLogger("test")
			if logger == nil {
				t.Error("Expected logger to be created")
				return
			}

			// Test that logger works with this format/output combination
			logger.Info("Test message", slog.String("format", string(tt.format)))
		})
	}
}

func TestFactory_FileOutput(t *testing.T) {
	// Create temporary file
	tempFile := filepath.Join(os.TempDir(), "factory_test.log")
	defer os.Remove(tempFile)

	config := &Config{
		Level:    LogLevelInfo,
		Format:   LogFormatJSON,
		Output:   LogOutputFile,
		FilePath: tempFile,
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory with file output: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("test")
	logger.Info("Test message to file", slog.String("test", "value"))

	// Give some time for async writes
	time.Sleep(100 * time.Millisecond)

	// Verify file exists and has content
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
		return
	}

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
		return
	}

	if len(content) == 0 {
		t.Error("Expected log file to have content")
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Test message to file") {
		t.Error("Expected log message to be in file")
	}
}

func TestFactory_AsyncLogging(t *testing.T) {
	config := &Config{
		Level:        LogLevelInfo,
		Format:       LogFormatJSON,
		Output:       LogOutputStdout,
		AsyncLogging: true,
		BufferSize:   1024,
	}

	factory, err := NewFactory(config)
	if err != nil {
		t.Fatalf("Failed to create factory with async logging: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("async-test")

	// Log multiple messages quickly
	for i := 0; i < 100; i++ {
		logger.Info("Async test message", slog.Int("id", i))
	}

	// Give some time for async processing
	time.Sleep(100 * time.Millisecond)
}

// Benchmark tests
func BenchmarkFactory_GetLogger(b *testing.B) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory.GetLogger("benchmark")
	}
}

func BenchmarkFactory_GetLoggerNew(b *testing.B) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		component := fmt.Sprintf("benchmark-%d", i)
		factory.GetLogger(component)
	}
}

func BenchmarkFactory_LogWithContext(b *testing.B) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("benchmark")
	ctx := CreateTestContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contextLogger := factory.WithContext(ctx, logger)
		contextLogger.Info("Benchmark message")
	}
}

func BenchmarkGlobalLogger(b *testing.B) {
	err := Initialize(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to initialize global factory: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := GetGlobalLogger("benchmark")
		logger.Info("Benchmark message")
	}
}