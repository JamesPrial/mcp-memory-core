package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkConfigValidation benchmarks configuration validation
func BenchmarkConfigValidation(b *testing.B) {
	config := DefaultConfig()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := config.Validate()
		if err != nil {
			b.Errorf("Validation failed: %v", err)
		}
	}
}

func BenchmarkConfigCreation(b *testing.B) {
	b.Run("Default", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DefaultConfig()
		}
	})

	b.Run("Development", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DevelopmentConfig()
		}
	})

	b.Run("Production", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ProductionConfig()
		}
	})
}

// BenchmarkFactoryCreation benchmarks factory creation with different configurations
func BenchmarkFactoryCreation(b *testing.B) {
	configs := map[string]*Config{
		"Minimal": {
			Level:  LogLevelError,
			Format: LogFormatText,
			Output: LogOutputStdout,
		},
		"Default": DefaultConfig(),
		"Full": {
			Level:           LogLevelDebug,
			Format:          LogFormatJSON,
			Output:          LogOutputStdout,
			AsyncLogging:    true,
			EnableRequestID: true,
			EnableTracing:   true,
			EnableAudit:     false, // Disable to avoid file creation in benchmarks
			Sampling: SamplingConfig{
				Enabled: true,
				Rate:    0.1,
			},
			Masking: MaskingConfig{
				Enabled:     true,
				MaskEmails:  true,
				MaskAPIKeys: true,
			},
			Metrics: MetricsConfig{
				Enabled: true,
			},
		},
	}

	for name, config := range configs {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				factory, err := NewFactory(config)
				if err != nil {
					b.Errorf("Factory creation failed: %v", err)
				}
				factory.Close()
			}
		})
	}
}

// BenchmarkLoggerCreation benchmarks logger creation from factory
func BenchmarkLoggerCreation(b *testing.B) {
	factory, err := NewFactory(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	b.Run("SameComponent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = factory.GetLogger("benchmark")
		}
	})

	b.Run("DifferentComponents", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			component := fmt.Sprintf("component-%d", i%100)
			_ = factory.GetLogger(component)
		}
	})

	b.Run("UniqueComponents", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			component := fmt.Sprintf("unique-component-%d", i)
			_ = factory.GetLogger(component)
		}
	})
}

// BenchmarkLogging benchmarks different logging scenarios
func BenchmarkLogging(b *testing.B) {
	configs := map[string]*Config{
		"Sync": {
			Level:        LogLevelInfo,
			Format:       LogFormatJSON,
			Output:       LogOutputStdout,
			AsyncLogging: false,
		},
		"Async": {
			Level:        LogLevelInfo,
			Format:       LogFormatJSON,
			Output:       LogOutputStdout,
			AsyncLogging: true,
			BufferSize:   4096,
		},
		"SampledLow": {
			Level:        LogLevelInfo,
			Format:       LogFormatJSON,
			Output:       LogOutputStdout,
			AsyncLogging: true,
			Sampling: SamplingConfig{
				Enabled: true,
				Rate:    0.1,
			},
		},
		"SampledHigh": {
			Level:        LogLevelInfo,
			Format:       LogFormatJSON,
			Output:       LogOutputStdout,
			AsyncLogging: true,
			Sampling: SamplingConfig{
				Enabled: true,
				Rate:    0.9,
			},
		},
	}

	for name, config := range configs {
		b.Run(name, func(b *testing.B) {
			factory, err := NewFactory(config)
			if err != nil {
				b.Fatalf("Failed to create factory: %v", err)
			}
			defer factory.Close()

			logger := factory.GetLogger("benchmark")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.Info("Benchmark log message",
					slog.Int("iteration", i),
					slog.String("component", "benchmark"),
					slog.Duration("elapsed", time.Duration(i)*time.Nanosecond),
				)
			}
		})
	}
}

// BenchmarkContextOperations benchmarks context operations
func BenchmarkContextOperations(b *testing.B) {
	b.Run("CreateRequestContext", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			_ = NewRequestContext(ctx, "benchmark")
		}
	})

	b.Run("ExtractRequestContext", func(b *testing.B) {
		ctx := CreateTestContext()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ExtractRequestContext(ctx)
		}
	})

	b.Run("InjectRequestContext", func(b *testing.B) {
		ctx := context.Background()
		rc := ExtractRequestContext(CreateTestContext())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = InjectRequestContext(ctx, rc)
		}
	})

	b.Run("GetRequestID", func(b *testing.B) {
		ctx := CreateTestContext()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = GetRequestID(ctx)
		}
	})

	b.Run("WithRequestID", func(b *testing.B) {
		ctx := context.Background()
		requestID := GenerateID()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = WithRequestID(ctx, requestID)
		}
	})

	b.Run("GetDuration", func(b *testing.B) {
		ctx := WithStartTime(context.Background(), time.Now())
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = GetDuration(ctx)
		}
	})
}

// BenchmarkIDGeneration benchmarks ID generation functions
func BenchmarkIDGeneration(b *testing.B) {
	b.Run("GenerateID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GenerateID()
		}
	})

	b.Run("GenerateTraceID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GenerateTraceID()
		}
	})

	b.Run("GenerateSpanID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GenerateSpanID()
		}
	})
}

// BenchmarkTraceParentOperations benchmarks W3C Trace Context operations
func BenchmarkTraceParentOperations(b *testing.B) {
	traceID := GenerateTraceID()
	spanID := GenerateSpanID()
	header := FormatTraceParent(traceID, spanID, true)

	b.Run("FormatTraceParent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = FormatTraceParent(traceID, spanID, i%2 == 0)
		}
	})

	b.Run("ParseTraceParent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, _ = ParseTraceParent(header)
		}
	})
}

// BenchmarkRequestInterceptor benchmarks request interception
func BenchmarkRequestInterceptor(b *testing.B) {
	factory, err := NewFactory(&Config{
		Level:        LogLevelInfo,
		Format:       LogFormatJSON,
		Output:       LogOutputStdout,
		AsyncLogging: true,
	})
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("benchmark")
	interceptor := NewRequestInterceptor(logger)

	b.Run("SuccessfulOperation", func(b *testing.B) {
		ctx := context.Background()
		fn := func(ctx context.Context) error {
			return nil
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interceptor.InterceptRequest(ctx, "benchmark", fn)
		}
	})

	b.Run("WithResponse", func(b *testing.B) {
		ctx := context.Background()
		fn := func(ctx context.Context) (interface{}, error) {
			return "result", nil
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = interceptor.InterceptWithResponse(ctx, "benchmark", fn)
		}
	})
}

// BenchmarkHTTPMiddleware benchmarks HTTP middleware performance
func BenchmarkHTTPMiddleware(b *testing.B) {
	factory, err := NewFactory(&Config{
		Level:        LogLevelInfo,
		Format:       LogFormatJSON,
		Output:       LogOutputStdout,
		AsyncLogging: true,
	})
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("benchmark")
	interceptor := NewRequestInterceptor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := interceptor.HTTPMiddleware(handler)

	b.Run("SimpleRequest", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/benchmark", nil)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)
		}
	})

	b.Run("WithHeaders", func(b *testing.B) {
		req := httptest.NewRequest("POST", "/benchmark", nil)
		req.Header.Set("User-Agent", "benchmark-client")
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:12345"
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)
		}
	})
}

// BenchmarkOperationTimer benchmarks operation timing
func BenchmarkOperationTimer(b *testing.B) {
	factory, err := NewFactory(&Config{
		Level:        LogLevelDebug,
		Format:       LogFormatJSON,
		Output:       LogOutputStdout,
		AsyncLogging: true,
	})
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("benchmark")
	ctx := CreateTestContext()

	b.Run("StartAndEnd", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timer := StartTimer(ctx, logger, "benchmark")
			timer.End()
		}
	})

	b.Run("StartAndEndWithError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timer := StartTimer(ctx, logger, "benchmark")
			timer.EndWithError(nil)
		}
	})

	b.Run("LogLatency", func(b *testing.B) {
		duration := 10 * time.Millisecond
		for i := 0; i < b.N; i++ {
			LogLatency(ctx, logger, "benchmark", duration, nil)
		}
	})
}

// BenchmarkMaskingOperations benchmarks data masking
func BenchmarkMaskingOperations(b *testing.B) {
	config := &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		Masking: MaskingConfig{
			Enabled:         true,
			Fields:          []string{"password", "secret", "token"},
			Patterns:        []string{`\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`},
			MaskEmails:      true,
			MaskPhoneNumbers: true,
			MaskCreditCards: true,
			MaskSSN:         true,
			MaskAPIKeys:     true,
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("benchmark")
	ctx := CreateTestContext()

	b.Run("WithMasking", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger.InfoContext(ctx, "User data",
				slog.String("email", "user@example.com"),
				slog.String("password", "secret123"),
				slog.String("api_key", "key_12345"),
				slog.String("phone", "+1-555-123-4567"),
			)
		}
	})

	// Compare with masking disabled
	configNoMask := &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		Masking: MaskingConfig{
			Enabled: false,
		},
	}

	factoryNoMask, err := NewFactory(configNoMask)
	if err != nil {
		b.Fatalf("Failed to create no-mask factory: %v", err)
	}
	defer factoryNoMask.Close()

	loggerNoMask := factoryNoMask.GetLogger("benchmark")

	b.Run("WithoutMasking", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			loggerNoMask.InfoContext(ctx, "User data",
				slog.String("email", "user@example.com"),
				slog.String("password", "secret123"),
				slog.String("api_key", "key_12345"),
				slog.String("phone", "+1-555-123-4567"),
			)
		}
	})
}

// BenchmarkSamplingOperations benchmarks sampling
func BenchmarkSamplingOperations(b *testing.B) {
	samplingRates := []float64{0.01, 0.1, 0.5, 0.9}

	for _, rate := range samplingRates {
		b.Run(fmt.Sprintf("Rate%.2f", rate), func(b *testing.B) {
			config := &Config{
				Level:        LogLevelInfo,
				Format:       LogFormatJSON,
				Output:       LogOutputStdout,
				AsyncLogging: true,
				Sampling: SamplingConfig{
					Enabled:      true,
					Rate:         rate,
					BurstSize:    100,
					AlwaysErrors: true,
				},
			}

			factory, err := NewFactory(config)
			if err != nil {
				b.Fatalf("Failed to create factory: %v", err)
			}
			defer factory.Close()

			logger := factory.GetLogger("benchmark")
			ctx := CreateTestContext()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.InfoContext(ctx, "Sampled log message",
					slog.Int("iteration", i),
					slog.String("data", "some data"),
				)
			}
		})
	}
}

// BenchmarkMetricsCollection benchmarks metrics operations
func BenchmarkMetricsCollection(b *testing.B) {
	config := &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		Metrics: MetricsConfig{
			Enabled:    true,
			Namespace:  "benchmark",
			Subsystem:  "test",
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	metrics := factory.GetMetricsCollector()
	if metrics == nil {
		b.Fatal("Expected metrics collector")
	}

	b.Run("RecordRequest", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics.RecordRequest("GET", "/api/test", "200", 10*time.Millisecond, 1024, 2048)
		}
	})

	b.Run("RecordComponentOperation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics.RecordComponentOperation("benchmark", "operation", 5*time.Millisecond)
		}
	})

	b.Run("RequestTimer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timer := metrics.NewRequestTimer("benchmark", "timer")
			timer.Finish(nil)
		}
	})

	b.Run("RecordCacheHit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics.RecordCacheHit("user_cache", "user_*")
		}
	})

	b.Run("RecordStorageQuery", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics.RecordStorageQuery("select", "users", 2*time.Millisecond)
		}
	})
}

// BenchmarkFileOperations benchmarks file-based logging
func BenchmarkFileOperations(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "benchmark.log")

	config := &Config{
		Level:        LogLevelInfo,
		Format:       LogFormatJSON,
		Output:       LogOutputFile,
		FilePath:     logFile,
		AsyncLogging: true,
		BufferSize:   8192,
	}

	factory, err := NewFactory(config)
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("benchmark")
	ctx := CreateTestContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "Benchmark file log",
			slog.Int("iteration", i),
			slog.String("component", "file-benchmark"),
			slog.Duration("elapsed", time.Duration(i)*time.Microsecond),
		)
	}

	// Give time for async writes
	time.Sleep(100 * time.Millisecond)

	// Verify file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		b.Error("Expected log file to be created")
	}
}

// BenchmarkAsyncVsSync compares async vs synchronous logging performance
func BenchmarkAsyncVsSync(b *testing.B) {
	b.Run("Sync", func(b *testing.B) {
		config := &Config{
			Level:        LogLevelInfo,
			Format:       LogFormatJSON,
			Output:       LogOutputStdout,
			AsyncLogging: false,
		}

		factory, err := NewFactory(config)
		if err != nil {
			b.Fatalf("Failed to create sync factory: %v", err)
		}
		defer factory.Close()

		logger := factory.GetLogger("sync-benchmark")
		ctx := CreateTestContext()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.InfoContext(ctx, "Sync benchmark message",
				slog.Int("iteration", i),
			)
		}
	})

	b.Run("Async", func(b *testing.B) {
		config := &Config{
			Level:        LogLevelInfo,
			Format:       LogFormatJSON,
			Output:       LogOutputStdout,
			AsyncLogging: true,
			BufferSize:   4096,
		}

		factory, err := NewFactory(config)
		if err != nil {
			b.Fatalf("Failed to create async factory: %v", err)
		}
		defer factory.Close()

		logger := factory.GetLogger("async-benchmark")
		ctx := CreateTestContext()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.InfoContext(ctx, "Async benchmark message",
				slog.Int("iteration", i),
			)
		}
	})
}

// BenchmarkComplexLogging benchmarks complex logging scenarios
func BenchmarkComplexLogging(b *testing.B) {
	config := &Config{
		Level:           LogLevelDebug,
		Format:          LogFormatJSON,
		Output:          LogOutputStdout,
		AsyncLogging:    true,
		EnableRequestID: true,
		EnableTracing:   true,
		EnableCaller:    true,
		Sampling: SamplingConfig{
			Enabled:   true,
			Rate:      0.5,
			BurstSize: 100,
		},
		Masking: MaskingConfig{
			Enabled:     true,
			MaskEmails:  true,
			MaskAPIKeys: true,
		},
		Metrics: MetricsConfig{
			Enabled: true,
		},
	}

	factory, err := NewFactory(config)
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("complex-benchmark")
	interceptor := NewRequestInterceptor(logger)

	b.Run("ComplexRequest", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			ctx = WithUserID(ctx, fmt.Sprintf("user-%d", i%1000))

			interceptor.InterceptRequest(ctx, "complex-operation", func(ctx context.Context) error {
				// Multiple log statements with various data types
				logger.DebugContext(ctx, "Starting complex operation",
					slog.String("user_email", "user@example.com"), // Will be masked
					slog.String("api_key", "key_secret123"),        // Will be masked
					slog.Int("iteration", i),
					slog.Duration("expected_duration", 10*time.Millisecond),
				)

				// Simulate nested operations
				timer := StartTimer(ctx, logger, "database-query")
				// Simulate work
				timer.End()

				logger.InfoContext(ctx, "Complex operation completed",
					slog.String("result", "success"),
					slog.Int("records_processed", i%100),
					slog.Bool("cache_hit", i%3 == 0),
				)

				return nil
			})
		}
	})
}

// BenchmarkConcurrentLogging benchmarks concurrent logging scenarios
func BenchmarkConcurrentLogging(b *testing.B) {
	config := &Config{
		Level:        LogLevelInfo,
		Format:       LogFormatJSON,
		Output:       LogOutputStdout,
		AsyncLogging: true,
		BufferSize:   8192,
	}

	factory, err := NewFactory(config)
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("concurrent-benchmark")

	b.Run("Sequential", func(b *testing.B) {
		ctx := CreateTestContext()
		for i := 0; i < b.N; i++ {
			logger.InfoContext(ctx, "Sequential log message",
				slog.Int("iteration", i),
			)
		}
	})

	b.Run("Concurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			ctx := CreateTestContext()
			i := 0
			for pb.Next() {
				logger.InfoContext(ctx, "Concurrent log message",
					slog.Int("iteration", i),
				)
				i++
			}
		})
	})
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	factory, err := NewFactory(&Config{
		Level:        LogLevelInfo,
		Format:       LogFormatJSON,
		Output:       LogOutputStdout,
		AsyncLogging: true,
	})
	if err != nil {
		b.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	logger := factory.GetLogger("memory-benchmark")

	b.Run("MinimalAllocation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("Simple message")
		}
	})

	b.Run("ModerateAllocation", func(b *testing.B) {
		ctx := CreateTestContext()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.InfoContext(ctx, "Message with context",
				slog.Int("id", i),
				slog.String("component", "benchmark"),
			)
		}
	})

	b.Run("HighAllocation", func(b *testing.B) {
		ctx := CreateTestContext()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.InfoContext(ctx, "Message with many attributes",
				slog.Int("id", i),
				slog.String("component", "benchmark"),
				slog.Duration("elapsed", time.Duration(i)*time.Nanosecond),
				slog.Bool("success", i%2 == 0),
				slog.Float64("score", float64(i)/100.0),
				slog.Time("timestamp", time.Now()),
				slog.String("description", fmt.Sprintf("Iteration %d description", i)),
			)
		}
	})
}