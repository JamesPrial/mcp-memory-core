package logging

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestGlobalErrorMetricsRace tests concurrent access to global error metrics
func TestGlobalErrorMetricsRace(t *testing.T) {
	ctx := context.Background()
	
	var wg sync.WaitGroup
	
	// Multiple goroutines recording errors concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				errorCode := fmt.Sprintf("ERR_%d", j%5)
				operation := fmt.Sprintf("op_%d", id)
				component := fmt.Sprintf("comp_%d", id%3)
				
				// Use the global RecordError function
				RecordError(ctx, errorCode, operation, component)
				
				// Also test GetGlobalMetrics
				metrics := GetGlobalMetrics()
				if metrics != nil {
					metrics.GetTotalErrors()
					metrics.GetErrorCounts()
				}
			}
		}(i)
	}
	
	// Goroutine getting metrics snapshots
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			metrics := GetGlobalMetrics()
			if metrics != nil {
				total := metrics.GetTotalErrors()
				_ = total
				counts := metrics.GetErrorCounts()
				_ = counts
			}
		}
	}()
	
	wg.Wait()
}

// TestGlobalFactoryRace tests concurrent access to global factory
func TestGlobalFactoryRace(t *testing.T) {
	// Initialize with a test config
	config := TestConfig()
	if err := Initialize(config); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer Shutdown()
	
	var wg sync.WaitGroup
	
	// Multiple goroutines getting loggers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				component := fmt.Sprintf("component_%d", id)
				logger := GetGlobalLogger(component)
				if logger != nil {
					logger.Info("test message", "id", id, "iteration", j)
				}
			}
		}(i)
	}
	
	// Goroutines getting other global resources
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				GetGlobalAuditLogger()
				GetGlobalMetricsCollector()
				GetGlobalMasker()
				GetGlobalSampler()
			}
		}()
	}
	
	// Goroutine updating log levels
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			UpdateGlobalLevel("test", LogLevelDebug)
			UpdateGlobalLevel("test", LogLevelInfo)
		}
	}()
	
	wg.Wait()
}