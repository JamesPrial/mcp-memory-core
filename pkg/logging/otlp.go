package logging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// OTLPHandler exports logs to OpenTelemetry collectors (simplified implementation)
type OTLPHandler struct {
	next   slog.Handler
	config OTLPConfig
	queue  chan *slog.Record
	wg     sync.WaitGroup
	done   chan struct{}
	
	// Health tracking
	healthy               bool
	connectionHealthy     bool
	lastExportTime        time.Time
	lastExportError       error
	exportAttempts        int64
	exportFailures        int64
	exportSuccesses       int64
	mu                   sync.RWMutex
}

// NewOTLPHandler creates a new OTLP export handler
func NewOTLPHandler(next slog.Handler, config OTLPConfig) (*OTLPHandler, error) {
	if !config.Enabled {
		return &OTLPHandler{next: next, healthy: true}, nil
	}
	
	h := &OTLPHandler{
		next:              next,
		config:            config,
		queue:             make(chan *slog.Record, config.QueueSize),
		done:              make(chan struct{}),
		healthy:           true,
		connectionHealthy: true,
		lastExportTime:    time.Now(),
	}
	
	// Start export goroutine
	h.wg.Add(1)
	go h.exportLoop()
	
	return h, nil
}

// Handle processes a log record
func (h *OTLPHandler) Handle(ctx context.Context, r slog.Record) error {
	// Pass through to next handler
	if err := h.next.Handle(ctx, r); err != nil {
		return err
	}
	
	if !h.config.Enabled {
		return nil
	}
	
	// Queue for export (non-blocking)
	select {
	case h.queue <- &r:
	default:
		// Queue full, drop log
		h.mu.Lock()
		h.exportFailures++
		h.mu.Unlock()
	}
	
	return nil
}

// WithAttrs returns a new handler with additional attributes
func (h *OTLPHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &OTLPHandler{
		next:              h.next.WithAttrs(attrs),
		config:            h.config,
		queue:             h.queue,
		done:              h.done,
		healthy:           h.healthy,
		connectionHealthy: h.connectionHealthy,
	}
}

// WithGroup returns a new handler with a group name
func (h *OTLPHandler) WithGroup(name string) slog.Handler {
	return &OTLPHandler{
		next:              h.next.WithGroup(name),
		config:            h.config,
		queue:             h.queue,
		done:              h.done,
		healthy:           h.healthy,
		connectionHealthy: h.connectionHealthy,
	}
}

// Enabled reports whether the handler is enabled for the given level
func (h *OTLPHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// exportLoop processes the export queue
func (h *OTLPHandler) exportLoop() {
	defer h.wg.Done()
	
	batch := make([]*slog.Record, 0, h.config.BatchSize)
	ticker := time.NewTicker(time.Duration(h.config.Timeout) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case record := <-h.queue:
			batch = append(batch, record)
			if len(batch) >= h.config.BatchSize {
				h.exportBatch(batch)
				batch = batch[:0]
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				h.exportBatch(batch)
				batch = batch[:0]
			}
			
		case <-h.done:
			// Export remaining logs
			if len(batch) > 0 {
				h.exportBatch(batch)
			}
			return
		}
	}
}

// exportBatch exports a batch of logs to OTLP (simplified stub)
func (h *OTLPHandler) exportBatch(batch []*slog.Record) {
	if !h.config.Enabled {
		return
	}
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.exportAttempts++
	
	// Simplified implementation - just log that we would export
	// In a real implementation, this would convert to OTLP format and send to endpoint
	fmt.Printf("OTLP: Would export %d logs to %s\n", len(batch), h.config.Endpoint)
	
	// Simulate success (in real implementation, check actual export result)
	h.exportSuccesses++
	h.lastExportTime = time.Now()
	h.connectionHealthy = true
	h.lastExportError = nil
}

// GetHealthStatus returns the current health status
func (h *OTLPHandler) GetHealthStatus() OTLPHealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return OTLPHealthStatus{
		Healthy:             h.healthy,
		ConnectionHealthy:   h.connectionHealthy,
		LastExportTime:      h.lastExportTime,
		LastError:           h.lastExportError,
		ExportAttempts:      h.exportAttempts,
		ExportSuccesses:     h.exportSuccesses,
		ExportFailures:      h.exportFailures,
	}
}

// GetExportStats returns export statistics
func (h *OTLPHandler) GetExportStats() ExportStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	successRate := 0.0
	if h.exportAttempts > 0 {
		successRate = float64(h.exportSuccesses) / float64(h.exportAttempts)
	}
	
	return ExportStats{
		TotalAttempts:     h.exportAttempts,
		TotalSuccesses:    h.exportSuccesses,
		TotalFailures:     h.exportFailures,
		SuccessRate:       successRate,
		LastExportTime:    h.lastExportTime,
		LastError:         h.lastExportError,
		QueueSize:         len(h.queue),
		QueueCapacity:     cap(h.queue),
		BufferUtilization: float64(len(h.queue)) / float64(cap(h.queue)),
	}
}

// Close gracefully shuts down the handler
func (h *OTLPHandler) Close() error {
	if h.done != nil {
		close(h.done)
		h.wg.Wait()
	}
	return nil
}