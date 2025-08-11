package logging

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ErrLoggerClosed is returned when trying to use a closed logger
var ErrLoggerClosed = errors.New("logger is closed")

// AsyncHandler provides buffered asynchronous logging
type AsyncHandler struct {
	handler     slog.Handler
	buffer      chan logEntry
	bufferSize  int
	flushPeriod time.Duration
	
	// State management
	closed    int32
	closeOnce sync.Once
	closeCh   chan struct{}
	doneCh    chan struct{}
	
	// Statistics
	stats AsyncStats
	
	// Error handling
	errorHandler func(error)
	
	// Flush control
	flushTicker *time.Ticker
	mu          sync.RWMutex
}

// logEntry represents a log entry to be processed asynchronously
type logEntry struct {
	ctx    context.Context
	record slog.Record
}

// AsyncStats provides statistics about the async handler
type AsyncStats struct {
	BufferedEntries  int64 `json:"buffered_entries"`
	ProcessedEntries int64 `json:"processed_entries"`
	DroppedEntries   int64 `json:"dropped_entries"`
	FlushCount       int64 `json:"flush_count"`
	ErrorCount       int64 `json:"error_count"`
	BufferSize       int   `json:"buffer_size"`
	BufferUsed       int   `json:"buffer_used"`
}

// AsyncConfig defines configuration for async handler
type AsyncConfig struct {
	BufferSize      int           `yaml:"bufferSize" json:"bufferSize"`
	FlushPeriod     time.Duration `yaml:"flushPeriod" json:"flushPeriod"`
	DropOnFull      bool          `yaml:"dropOnFull" json:"dropOnFull"`
	FlushOnClose    bool          `yaml:"flushOnClose" json:"flushOnClose"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" json:"shutdownTimeout"`
}

// DefaultAsyncConfig returns default async handler configuration
func DefaultAsyncConfig() AsyncConfig {
	return AsyncConfig{
		BufferSize:      1000,
		FlushPeriod:     5 * time.Second,
		DropOnFull:      false,
		FlushOnClose:    true,
		ShutdownTimeout: 30 * time.Second,
	}
}

// NewAsyncHandler creates a new asynchronous log handler
func NewAsyncHandler(handler slog.Handler, bufferSize int) *AsyncHandler {
	return NewAsyncHandlerWithConfig(handler, AsyncConfig{
		BufferSize:  bufferSize,
		FlushPeriod: 5 * time.Second,
	})
}

// NewAsyncHandlerWithConfig creates a new async handler with custom configuration
func NewAsyncHandlerWithConfig(handler slog.Handler, config AsyncConfig) *AsyncHandler {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.FlushPeriod <= 0 {
		config.FlushPeriod = 5 * time.Second
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	ah := &AsyncHandler{
		handler:     handler,
		buffer:      make(chan logEntry, config.BufferSize),
		bufferSize:  config.BufferSize,
		flushPeriod: config.FlushPeriod,
		closeCh:     make(chan struct{}),
		doneCh:      make(chan struct{}),
		errorHandler: func(err error) {
			// Default error handler - could log to stderr or another handler
		},
	}

	ah.stats.BufferSize = config.BufferSize
	
	// Start background processing goroutine
	go ah.processLoop()
	
	// Start periodic flush if configured
	if config.FlushPeriod > 0 {
		ah.flushTicker = time.NewTicker(config.FlushPeriod)
		go ah.flushLoop()
	}

	return ah
}

// Handle implements slog.Handler interface
func (ah *AsyncHandler) Handle(ctx context.Context, record slog.Record) error {
	if atomic.LoadInt32(&ah.closed) != 0 {
		return ErrLoggerClosed
	}

	entry := logEntry{
		ctx:    ctx,
		record: record.Clone(),
	}

	select {
	case ah.buffer <- entry:
		atomic.AddInt64(&ah.stats.BufferedEntries, 1)
		return nil
	default:
		// Buffer is full
		atomic.AddInt64(&ah.stats.DroppedEntries, 1)
		return nil // Don't block the caller
	}
}

// WithAttrs implements slog.Handler interface
func (ah *AsyncHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &AsyncHandler{
		handler:      ah.handler.WithAttrs(attrs),
		buffer:       ah.buffer,
		bufferSize:   ah.bufferSize,
		flushPeriod:  ah.flushPeriod,
		closed:       ah.closed,
		closeCh:      ah.closeCh,
		doneCh:       ah.doneCh,
		stats:        ah.stats,
		errorHandler: ah.errorHandler,
		flushTicker:  ah.flushTicker,
	}
}

// WithGroup implements slog.Handler interface
func (ah *AsyncHandler) WithGroup(name string) slog.Handler {
	return &AsyncHandler{
		handler:      ah.handler.WithGroup(name),
		buffer:       ah.buffer,
		bufferSize:   ah.bufferSize,
		flushPeriod:  ah.flushPeriod,
		closed:       ah.closed,
		closeCh:      ah.closeCh,
		doneCh:       ah.doneCh,
		stats:        ah.stats,
		errorHandler: ah.errorHandler,
		flushTicker:  ah.flushTicker,
	}
}

// Enabled implements slog.Handler interface
func (ah *AsyncHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return ah.handler.Enabled(ctx, level)
}

// processLoop runs the main processing loop in a background goroutine
func (ah *AsyncHandler) processLoop() {
	defer close(ah.doneCh)

	for {
		select {
		case entry, ok := <-ah.buffer:
			if !ok {
				// Buffer channel closed, drain remaining entries
				ah.drainBuffer()
				return
			}
			
			if err := ah.handler.Handle(entry.ctx, entry.record); err != nil {
				atomic.AddInt64(&ah.stats.ErrorCount, 1)
				if ah.errorHandler != nil {
					ah.errorHandler(err)
				}
			} else {
				atomic.AddInt64(&ah.stats.ProcessedEntries, 1)
			}
			
		case <-ah.closeCh:
			// Close signal received, drain buffer and exit
			ah.drainBuffer()
			return
		}
	}
}

// flushLoop runs periodic flush operations
func (ah *AsyncHandler) flushLoop() {
	if ah.flushTicker == nil {
		return
	}

	for {
		select {
		case <-ah.flushTicker.C:
			ah.Flush()
		case <-ah.closeCh:
			ah.flushTicker.Stop()
			return
		}
	}
}

// drainBuffer processes any remaining entries in the buffer
func (ah *AsyncHandler) drainBuffer() {
	for {
		select {
		case entry, ok := <-ah.buffer:
			if !ok {
				return
			}
			if err := ah.handler.Handle(entry.ctx, entry.record); err != nil {
				atomic.AddInt64(&ah.stats.ErrorCount, 1)
				if ah.errorHandler != nil {
					ah.errorHandler(err)
				}
			} else {
				atomic.AddInt64(&ah.stats.ProcessedEntries, 1)
			}
		default:
			return
		}
	}
}

// Flush forces processing of all buffered log entries
func (ah *AsyncHandler) Flush() {
	if atomic.LoadInt32(&ah.closed) != 0 {
		return
	}

	// Send a flush signal by temporarily closing and reopening a channel
	// This is a simple approach - in production you might want a more sophisticated method
	atomic.AddInt64(&ah.stats.FlushCount, 1)
	
	// Wait for buffer to be processed (simple approach)
	for len(ah.buffer) > 0 {
		time.Sleep(1 * time.Millisecond)
	}
}

// Close closes the async handler and waits for all buffered entries to be processed
func (ah *AsyncHandler) Close() error {
	ah.closeOnce.Do(func() {
		atomic.StoreInt32(&ah.closed, 1)
		
		// Stop the flush ticker
		if ah.flushTicker != nil {
			ah.flushTicker.Stop()
		}
		
		// Signal close to processing goroutines
		close(ah.closeCh)
		
		// Close the buffer channel
		close(ah.buffer)
		
		// Wait for processing to complete with timeout
		select {
		case <-ah.doneCh:
			// Processing completed
		case <-time.After(30 * time.Second):
			// Timeout - force close
		}
	})
	
	return nil
}

// GetStats returns current statistics about the async handler
func (ah *AsyncHandler) GetStats() AsyncStats {
	ah.mu.RLock()
	defer ah.mu.RUnlock()

	stats := ah.stats
	stats.BufferUsed = len(ah.buffer)
	stats.BufferedEntries = atomic.LoadInt64(&ah.stats.BufferedEntries)
	stats.ProcessedEntries = atomic.LoadInt64(&ah.stats.ProcessedEntries)
	stats.DroppedEntries = atomic.LoadInt64(&ah.stats.DroppedEntries)
	stats.FlushCount = atomic.LoadInt64(&ah.stats.FlushCount)
	stats.ErrorCount = atomic.LoadInt64(&ah.stats.ErrorCount)
	
	return stats
}

// SetErrorHandler sets a custom error handler for processing errors
func (ah *AsyncHandler) SetErrorHandler(handler func(error)) {
	ah.mu.Lock()
	defer ah.mu.Unlock()
	ah.errorHandler = handler
}

// IsClosed returns whether the handler is closed
func (ah *AsyncHandler) IsClosed() bool {
	return atomic.LoadInt32(&ah.closed) != 0
}

// BufferUtilization returns the current buffer utilization as a percentage
func (ah *AsyncHandler) BufferUtilization() float64 {
	used := len(ah.buffer)
	if ah.bufferSize == 0 {
		return 0
	}
	return float64(used) / float64(ah.bufferSize) * 100
}

// IsHealthy returns whether the async handler is operating within healthy parameters
func (ah *AsyncHandler) IsHealthy() bool {
	if ah.IsClosed() {
		return false
	}

	utilization := ah.BufferUtilization()
	
	// Consider unhealthy if buffer is >90% full
	if utilization > 90 {
		return false
	}

	// Consider unhealthy if error rate is too high
	stats := ah.GetStats()
	if stats.ProcessedEntries > 0 {
		errorRate := float64(stats.ErrorCount) / float64(stats.ProcessedEntries)
		if errorRate > 0.1 { // 10% error rate threshold
			return false
		}
	}

	return true
}

// HealthStatus provides detailed health information
type HealthStatus struct {
	Healthy           bool    `json:"healthy"`
	Closed            bool    `json:"closed"`
	BufferUtilization float64 `json:"buffer_utilization"`
	ErrorRate         float64 `json:"error_rate"`
	Stats             AsyncStats `json:"stats"`
}

// GetHealthStatus returns detailed health status
func (ah *AsyncHandler) GetHealthStatus() HealthStatus {
	stats := ah.GetStats()
	
	errorRate := 0.0
	if stats.ProcessedEntries > 0 {
		errorRate = float64(stats.ErrorCount) / float64(stats.ProcessedEntries)
	}

	return HealthStatus{
		Healthy:           ah.IsHealthy(),
		Closed:            ah.IsClosed(),
		BufferUtilization: ah.BufferUtilization(),
		ErrorRate:         errorRate,
		Stats:             stats,
	}
}

// ResizeBuffer dynamically resizes the buffer (creates a new handler)
func (ah *AsyncHandler) ResizeBuffer(newSize int) *AsyncHandler {
	if newSize <= 0 {
		newSize = 1000
	}

	config := AsyncConfig{
		BufferSize:  newSize,
		FlushPeriod: ah.flushPeriod,
	}

	return NewAsyncHandlerWithConfig(ah.handler, config)
}

// BatchingAsyncHandler provides batched asynchronous logging
type BatchingAsyncHandler struct {
	*AsyncHandler
	batchSize     int
	batchTimeout  time.Duration
	currentBatch  []logEntry
	batchMu       sync.Mutex
	batchTimer    *time.Timer
}

// NewBatchingAsyncHandler creates a new batching async handler
func NewBatchingAsyncHandler(handler slog.Handler, batchSize int, batchTimeout time.Duration) *BatchingAsyncHandler {
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchTimeout <= 0 {
		batchTimeout = 1 * time.Second
	}

	asyncHandler := NewAsyncHandler(handler, 1000)
	
	bah := &BatchingAsyncHandler{
		AsyncHandler: asyncHandler,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		currentBatch: make([]logEntry, 0, batchSize),
	}

	return bah
}

// Handle implements slog.Handler interface with batching
func (bah *BatchingAsyncHandler) Handle(ctx context.Context, record slog.Record) error {
	if bah.IsClosed() {
		return ErrLoggerClosed
	}

	entry := logEntry{
		ctx:    ctx,
		record: record.Clone(),
	}

	bah.batchMu.Lock()
	defer bah.batchMu.Unlock()

	bah.currentBatch = append(bah.currentBatch, entry)

	// Process batch if full
	if len(bah.currentBatch) >= bah.batchSize {
		return bah.processBatch()
	}

	// Set or reset the batch timer
	if bah.batchTimer != nil {
		bah.batchTimer.Stop()
	}
	bah.batchTimer = time.AfterFunc(bah.batchTimeout, func() {
		bah.batchMu.Lock()
		defer bah.batchMu.Unlock()
		bah.processBatch()
	})

	return nil
}

// processBatch processes the current batch of log entries
func (bah *BatchingAsyncHandler) processBatch() error {
	if len(bah.currentBatch) == 0 {
		return nil
	}

	// Process each entry in the batch
	for _, entry := range bah.currentBatch {
		if err := bah.AsyncHandler.Handle(entry.ctx, entry.record); err != nil {
			return err
		}
	}

	// Clear the batch
	bah.currentBatch = bah.currentBatch[:0]

	return nil
}

// Close closes the batching handler and flushes any remaining batch
func (bah *BatchingAsyncHandler) Close() error {
	bah.batchMu.Lock()
	defer bah.batchMu.Unlock()

	// Process any remaining batch
	bah.processBatch()

	// Stop the batch timer
	if bah.batchTimer != nil {
		bah.batchTimer.Stop()
	}

	return bah.AsyncHandler.Close()
}