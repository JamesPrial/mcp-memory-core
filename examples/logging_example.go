package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
)

func main() {
	// Example demonstrating Phase 4 logging implementation
	fmt.Println("=== MCP Memory Core - Phase 4 Logging Example ===")

	// 1. Configure comprehensive logging
	config := logging.DefaultConfig()
	config.Level = logging.LogLevelDebug
	config.Format = logging.LogFormatJSON
	config.AsyncLogging = true
	config.EnableAudit = true
	config.AuditFilePath = "example_audit.log"
	
	// Enable metrics collection
	config.Metrics.Enabled = true
	config.Metrics.ListenAddr = ":9091"
	config.Metrics.EnableRuntime = true
	
	// Enable sampling for high-volume scenarios
	config.Sampling.Enabled = true
	config.Sampling.Rate = 0.8 // Sample 80% of logs
	config.Sampling.AlwaysErrors = true
	
	// Enable data masking
	config.Masking.Enabled = true
	config.Masking.MaskEmails = true
	config.Masking.MaskPhoneNumbers = true
	
	fmt.Printf("Initialized logging configuration with metrics, audit, and sampling\n")

	// 2. Initialize the logging factory
	factory, err := logging.NewFactory(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logging factory: %v", err))
	}
	defer factory.Close()

	// Initialize global factory for convenience
	if err := logging.Initialize(config); err != nil {
		panic(fmt.Sprintf("Failed to initialize global logging: %v", err))
	}

	fmt.Println("Logging factory initialized successfully")

	// 3. Demonstrate component-specific logging
	storageLogger := factory.GetLogger("storage")
	apiLogger := factory.GetLogger("api")
	knowledgeLogger := factory.GetLogger("knowledge")

	// 4. Create context with request metadata
	ctx := context.Background()
	ctx = logging.WithRequestID(ctx, "req-12345")
	ctx = logging.WithUserID(ctx, "user-67890")
	ctx = logging.WithOperation(ctx, "entity.create")

	// 5. Demonstrate basic logging with context
	fmt.Println("\n=== Basic Logging Examples ===")
	
	storageLogger.InfoContext(ctx, "Database connection established",
		slog.String("database", "sqlite"),
		slog.String("path", "/tmp/memory.db"))

	apiLogger.WarnContext(ctx, "API rate limit approaching",
		slog.Int("current_requests", 950),
		slog.Int("limit", 1000))

	// 6. Demonstrate data masking
	fmt.Println("\n=== Data Masking Examples ===")
	
	masker := factory.GetMasker()
	if masker != nil {
		sensitiveData := "User email: john.doe@example.com, phone: 555-123-4567, SSN: 123-45-6789"
		maskedData := masker.MaskString(sensitiveData)
		fmt.Printf("Original: %s\n", sensitiveData)
		fmt.Printf("Masked:   %s\n", maskedData)
	}

	// 7. Demonstrate audit logging
	fmt.Println("\n=== Audit Logging Examples ===")
	
	auditLogger := factory.GetAuditLogger()
	if auditLogger != nil {
		// Log entity creation
		err := auditLogger.LogEntityCreated(ctx, "person", "entity-123", map[string]interface{}{
			"name": "John Doe",
			"type": "person",
		})
		if err != nil {
			fmt.Printf("Audit logging failed: %v\n", err)
		} else {
			fmt.Println("Entity creation audit event logged")
		}

		// Log entity update with before/after changes
		changes := &logging.Changes{
			Before: map[string]interface{}{"status": "draft"},
			After:  map[string]interface{}{"status": "published"},
			Fields: []string{"status"},
		}
		
		err = auditLogger.LogEntityUpdated(ctx, "document", "doc-456", changes)
		if err != nil {
			fmt.Printf("Audit logging failed: %v\n", err)
		} else {
			fmt.Println("Entity update audit event logged")
		}

		// Log access attempt
		err = auditLogger.LogAccessAttempt(ctx, "knowledge_graph", "read", "success", 150*time.Millisecond)
		if err != nil {
			fmt.Printf("Audit logging failed: %v\n", err)
		} else {
			fmt.Println("Access attempt audit event logged")
		}
	}

	// 8. Demonstrate performance metrics
	fmt.Println("\n=== Performance Metrics Examples ===")
	
	metricsCollector := factory.GetMetricsCollector()
	if metricsCollector != nil {
		// Record some example operations
		metricsCollector.RecordComponentOperation("storage", "query", 50*time.Millisecond)
		metricsCollector.RecordComponentOperation("knowledge", "search", 200*time.Millisecond)
		metricsCollector.RecordStorageQuery("SELECT", "entities", 25*time.Millisecond)
		metricsCollector.RecordCacheHit("entity", "person:*")
		metricsCollector.RecordCacheMiss("relation", "knows:*")
		
		fmt.Println("Performance metrics recorded")
		
		// Start metrics server in background (commented out to avoid blocking)
		// go metricsCollector.StartMetricsServer()
		// fmt.Println("Metrics server started on :9091/metrics")
	}

	// 9. Demonstrate high-volume logging with sampling
	fmt.Println("\n=== High-Volume Logging with Sampling ===")
	
	sampler := factory.GetSampler()
	if sampler != nil {
		// Simulate high-volume logging
		for i := 0; i < 100; i++ {
			if sampler.ShouldSample(slog.LevelInfo) {
				knowledgeLogger.InfoContext(ctx, "Processing knowledge graph operation",
					slog.Int("iteration", i),
					slog.String("operation", "entity_relation_analysis"))
			}
		}
		
		stats := sampler.GetStats()
		fmt.Printf("Sampling stats: Seen=%d, Sampled=%d, Dropped=%d, Rate=%.2f\n",
			stats.TotalSeen, stats.TotalSampled, stats.TotalDropped, stats.SampleRate)
	}

	// 10. Demonstrate error logging with stack traces
	fmt.Println("\n=== Error Logging Examples ===")
	
	// Enable stack traces for errors
	config.EnableStackTrace = true
	
	// Simulate an error scenario
	err = fmt.Errorf("simulated database connection error")
	storageLogger.ErrorContext(ctx, "Database operation failed",
		slog.String("error", err.Error()),
		slog.String("query", "SELECT * FROM entities WHERE type = ?"),
		slog.Any("parameters", []string{"person"}))

	// Log component error through metrics
	if metricsCollector != nil {
		metricsCollector.RecordComponentError("storage", "database_query", "connection_error")
		fmt.Println("Component error recorded in metrics")
	}

	// 11. Demonstrate custom log levels per component
	fmt.Println("\n=== Component-Level Configuration ===")
	
	// Set different log levels for different components
	factory.UpdateLevel("storage", logging.LogLevelWarn)  // Only warnings and errors
	factory.UpdateLevel("api", logging.LogLevelDebug)     // All log levels
	
	// These logs will behave according to their component levels
	storageLogger.DebugContext(ctx, "This debug message will be filtered out")
	storageLogger.WarnContext(ctx, "This warning message will appear")
	
	apiLogger.DebugContext(ctx, "This debug message will appear")

	// 12. Demonstrate request timing and performance tracking
	fmt.Println("\n=== Request Timing Examples ===")
	
	if metricsCollector != nil {
		// Time a request using the timer utility
		timer := metricsCollector.NewRequestTimer("api", "create_entity")
		
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		// Finish timing (records duration automatically)
		timer.Finish(nil)
		
		// Or use the convenience method
		err := metricsCollector.TimeOperation("knowledge", "graph_traversal", func() error {
			// Simulate graph traversal work
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			fmt.Printf("Timed operation failed: %v\n", err)
		} else {
			fmt.Println("Graph traversal operation timed successfully")
		}
	}

	fmt.Println("\n=== Phase 4 Logging Demo Complete ===")
	fmt.Println("Check the following outputs:")
	fmt.Println("- Console: Structured JSON logs with context")
	fmt.Println("- example_audit.log: Tamper-resistant audit trail")
	fmt.Println("- :9091/metrics: Prometheus metrics endpoint (if enabled)")
	fmt.Println("- Masked sensitive data in all log outputs")
}