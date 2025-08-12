package errors

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
)

// TestDefaultLoggerRace tests concurrent access to default logger
func TestDefaultLoggerRace(t *testing.T) {
	ctx := context.Background()
	
	var wg sync.WaitGroup
	
	// Multiple goroutines using global logging functions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				err := fmt.Errorf("test error %d-%d", id, j)
				operation := fmt.Sprintf("operation_%d", id)
				
				// Test LogError
				LogError(ctx, err, operation)
				
				// Test LogAndWrap
				appErr := LogAndWrap(ctx, err, ErrCodeValidationInvalid, "test message", operation)
				_ = appErr
				
				// Test LogAndWrapf
				LogAndWrapf(ctx, err, ErrCodeValidationInvalid, operation, "formatted %d", j)
			}
		}(i)
	}
	
	// Goroutine setting default logger
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			logger := slog.Default()
			SetDefaultLogger(logger)
			
			// Also test SetDefaultLoggerComponent
			if i%2 == 0 {
				SetDefaultLoggerComponent(fmt.Sprintf("component_%d", i))
			}
		}
	}()
	
	// Test panic logging
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				recovered := fmt.Sprintf("panic %d-%d", id, j)
				LogPanic(ctx, recovered, "panic_operation")
			}
		}(i)
	}
	
	wg.Wait()
}

// TestLoggerMethodsRace tests concurrent access to Logger methods
func TestLoggerMethodsRace(t *testing.T) {
	logger := NewLogger("test")
	ctx := context.Background()
	
	var wg sync.WaitGroup
	
	// Multiple goroutines using the same logger instance
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				err := fmt.Errorf("error %d-%d", id, j)
				operation := fmt.Sprintf("op_%d", id)
				
				// Various logger operations
				logger.LogError(ctx, err, operation)
				logger.LogAndWrap(ctx, err, ErrCodeValidationInvalid, "message", operation)
				logger.LogAndWrapf(ctx, err, ErrCodeValidationInvalid, operation, "format %d", j)
				
				// Test panic recovery
				if j%10 == 0 {
					logger.LogPanic(ctx, "test panic", operation)
				}
			}
		}(i)
	}
	
	wg.Wait()
}