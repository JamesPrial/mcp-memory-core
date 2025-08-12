package logging

import (
	"context"
	"log/slog"
)

// SamplingHandler provides log sampling functionality
type SamplingHandler struct {
	handler slog.Handler
	sampler *Sampler
}

// NewSamplingHandler creates a new sampling handler
func NewSamplingHandler(handler slog.Handler, sampler *Sampler) *SamplingHandler {
	return &SamplingHandler{
		handler: handler,
		sampler: sampler,
	}
}

// Enabled implements slog.Handler
func (h *SamplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler
func (h *SamplingHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.sampler.ShouldSample(record.Level) {
		return h.handler.Handle(ctx, record)
	}
	return nil
}

// WithAttrs implements slog.Handler
func (h *SamplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SamplingHandler{
		handler: h.handler.WithAttrs(attrs),
		sampler: h.sampler,
	}
}

// WithGroup implements slog.Handler
func (h *SamplingHandler) WithGroup(name string) slog.Handler {
	return &SamplingHandler{
		handler: h.handler.WithGroup(name),
		sampler: h.sampler,
	}
}