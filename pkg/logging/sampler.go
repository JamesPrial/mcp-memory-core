package logging

import (
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// Sampler provides log sampling functionality
type Sampler struct {
	config       SamplingConfig
	mu           sync.Mutex
	counter      int
	window       time.Time
	rand         *rand.Rand
	totalSeen    int64
	totalSampled int64
	totalDropped int64
}

// NewSampler creates a new sampler
func NewSampler(config SamplingConfig) *Sampler {
	return &Sampler{
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
		window: time.Now(),
	}
}

// ShouldSample determines if a log record should be sampled
func (s *Sampler) ShouldSample(level slog.Level) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.totalSeen++
	
	if !s.config.Enabled {
		s.totalSampled++
		return true
	}
	
	// Always log errors if configured
	if s.config.AlwaysErrors && level >= slog.LevelError {
		s.totalSampled++
		return true
	}
	
	now := time.Now()
	
	// Reset counter every minute
	if now.Sub(s.window) > time.Minute {
		s.counter = 0
		s.window = now
	}
	
	// Allow burst size number of logs without sampling
	if s.counter < s.config.BurstSize {
		s.counter++
		s.totalSampled++
		return true
	}
	
	// Apply sampling rate
	if s.rand.Float64() < s.config.Rate {
		s.totalSampled++
		return true
	}
	
	s.totalDropped++
	return false
}

// SamplingStats provides statistics about sampling operations
type SamplingStats struct {
	TotalSeen    int64         `json:"total_seen"`
	TotalSampled int64         `json:"total_sampled"`
	TotalDropped int64         `json:"total_dropped"`
	SampleRate   float64       `json:"actual_sample_rate"`
	Config       SamplingConfig `json:"config"`
}

// GetStats returns sampling statistics
func (s *Sampler) GetStats() SamplingStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	sampleRate := 0.0
	if s.totalSeen > 0 {
		sampleRate = float64(s.totalSampled) / float64(s.totalSeen)
	}
	
	return SamplingStats{
		TotalSeen:    s.totalSeen,
		TotalSampled: s.totalSampled,
		TotalDropped: s.totalDropped,
		SampleRate:   sampleRate,
		Config:       s.config,
	}
}