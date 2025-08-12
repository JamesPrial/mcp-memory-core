package logging

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AdvancedSampler provides sophisticated sampling strategies
type AdvancedSampler struct {
	strategies      []SamplingStrategy
	defaultStrategy SamplingStrategy
	mu              sync.RWMutex
	stats           SamplingStrategyStats
	config          AdvancedSamplingConfig
}

// AdvancedSamplingConfig defines configuration for advanced sampling
type AdvancedSamplingConfig struct {
	Enabled                 bool                         `yaml:"enabled" json:"enabled"`
	DefaultStrategy         string                       `yaml:"defaultStrategy" json:"defaultStrategy"`
	SystemLoadThreshold     float64                      `yaml:"systemLoadThreshold" json:"systemLoadThreshold"`
	MemoryThresholdPercent  float64                      `yaml:"memoryThresholdPercent" json:"memoryThresholdPercent"`
	AdaptiveEnabled         bool                         `yaml:"adaptiveEnabled" json:"adaptiveEnabled"`
	AdaptiveInterval        int                          `yaml:"adaptiveInterval" json:"adaptiveInterval"` // seconds
	PriorityRules           []PrioritySamplingRule       `yaml:"priorityRules,omitempty" json:"priorityRules,omitempty"`
	TailBasedConfig         TailBasedSamplingConfig      `yaml:"tailBased,omitempty" json:"tailBased,omitempty"`
	StrategyConfigs         map[string]interface{}       `yaml:"strategyConfigs,omitempty" json:"strategyConfigs,omitempty"`
}

// SamplingStrategy defines a sampling strategy interface
type SamplingStrategy interface {
	Name() string
	ShouldSample(level slog.Level, component string, attrs map[string]interface{}) bool
	UpdateConfig(config interface{}) error
	GetStats() SamplingStrategyStats
	Reset()
}

// SamplingStrategyStats provides statistics for sampling strategies
type SamplingStrategyStats struct {
	Name                string    `json:"name"`
	TotalEvaluated      int64     `json:"total_evaluated"`
	TotalSampled        int64     `json:"total_sampled"`
	TotalDropped        int64     `json:"total_dropped"`
	SampleRate          float64   `json:"sample_rate"`
	LastEvaluation      time.Time `json:"last_evaluation"`
	LastConfigUpdate    time.Time `json:"last_config_update"`
	CurrentThreshold    float64   `json:"current_threshold,omitempty"`
}

// PrioritySamplingRule defines rules for priority-based sampling
type PrioritySamplingRule struct {
	Name        string                 `yaml:"name" json:"name"`
	Conditions  []SamplingCondition    `yaml:"conditions" json:"conditions"`
	SampleRate  float64                `yaml:"sampleRate" json:"sampleRate"`
	Priority    int                    `yaml:"priority" json:"priority"`
	AlwaysSample bool                  `yaml:"alwaysSample" json:"alwaysSample"`
}

// SamplingCondition defines a condition for sampling decisions
type SamplingCondition struct {
	Field    string      `yaml:"field" json:"field"`
	Operator string      `yaml:"operator" json:"operator"` // "eq", "ne", "contains", "gt", "lt", "gte", "lte"
	Value    interface{} `yaml:"value" json:"value"`
}

// TailBasedSamplingConfig defines configuration for tail-based sampling
type TailBasedSamplingConfig struct {
	Enabled              bool    `yaml:"enabled" json:"enabled"`
	BufferSize           int     `yaml:"bufferSize" json:"bufferSize"`
	TraceTimeoutSeconds  int     `yaml:"traceTimeoutSeconds" json:"traceTimeoutSeconds"`
	ErrorSampleRate      float64 `yaml:"errorSampleRate" json:"errorSampleRate"`
	SlowTraceSampleRate  float64 `yaml:"slowTraceSampleRate" json:"slowTraceSampleRate"`
	SlowTraceThresholdMs int64   `yaml:"slowTraceThresholdMs" json:"slowTraceThresholdMs"`
	DefaultSampleRate    float64 `yaml:"defaultSampleRate" json:"defaultSampleRate"`
}

// NewAdvancedSampler creates a new advanced sampler
func NewAdvancedSampler(config AdvancedSamplingConfig) *AdvancedSampler {
	sampler := &AdvancedSampler{
		config:     config,
		strategies: make([]SamplingStrategy, 0),
	}
	
	// Initialize default strategies
	sampler.initializeStrategies()
	
	// Start adaptive sampling if enabled
	if config.AdaptiveEnabled {
		go sampler.adaptiveSamplingLoop()
	}
	
	return sampler
}

// initializeStrategies initializes the built-in sampling strategies
func (as *AdvancedSampler) initializeStrategies() {
	// Fixed rate strategy
	fixedRate := &FixedRateSamplingStrategy{
		rate: 1.0, // Default to sampling everything
	}
	as.strategies = append(as.strategies, fixedRate)
	
	// Adaptive load-based strategy
	if as.config.AdaptiveEnabled {
		adaptive := &AdaptiveLoadSamplingStrategy{
			config: AdaptiveLoadConfig{
				InitialRate:             1.0,
				MinRate:                 0.01,
				MaxRate:                 1.0,
				SystemLoadThreshold:     as.config.SystemLoadThreshold,
				MemoryThresholdPercent:  as.config.MemoryThresholdPercent,
				AdjustmentInterval:      time.Duration(as.config.AdaptiveInterval) * time.Second,
			},
			currentRate: 1.0,
		}
		as.strategies = append(as.strategies, adaptive)
		as.defaultStrategy = adaptive
	} else {
		as.defaultStrategy = fixedRate
	}
	
	// Priority-based strategy
	if len(as.config.PriorityRules) > 0 {
		priority := &PriorityBasedSamplingStrategy{
			rules: as.config.PriorityRules,
		}
		as.strategies = append(as.strategies, priority)
		as.defaultStrategy = priority
	}
	
	// Tail-based strategy
	if as.config.TailBasedConfig.Enabled {
		tailBased := NewTailBasedSamplingStrategy(as.config.TailBasedConfig)
		as.strategies = append(as.strategies, tailBased)
	}
}

// ShouldSample determines if a log record should be sampled
func (as *AdvancedSampler) ShouldSample(level slog.Level, component string, attrs map[string]interface{}) bool {
	if !as.config.Enabled {
		return true
	}
	
	as.mu.RLock()
	defer as.mu.RUnlock()
	
	// Use default strategy for primary decision
	shouldSample := as.defaultStrategy.ShouldSample(level, component, attrs)
	
	// Apply other strategies as overrides
	for _, strategy := range as.strategies {
		if strategy == as.defaultStrategy {
			continue
		}
		
		// Some strategies might override the default decision
		if overrideSample := strategy.ShouldSample(level, component, attrs); overrideSample != shouldSample {
			// Priority-based rules can override
			if _, ok := strategy.(*PriorityBasedSamplingStrategy); ok {
				shouldSample = overrideSample
			}
		}
	}
	
	// Update stats
	as.stats.TotalEvaluated++
	if shouldSample {
		as.stats.TotalSampled++
	} else {
		as.stats.TotalDropped++
	}
	as.stats.LastEvaluation = time.Now()
	
	if as.stats.TotalEvaluated > 0 {
		as.stats.SampleRate = float64(as.stats.TotalSampled) / float64(as.stats.TotalEvaluated)
	}
	
	return shouldSample
}

// adaptiveSamplingLoop adjusts sampling rates based on system conditions
func (as *AdvancedSampler) adaptiveSamplingLoop() {
	interval := time.Duration(as.config.AdaptiveInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for range ticker.C {
		as.adjustSamplingRates()
	}
}

// adjustSamplingRates adjusts sampling rates based on current system conditions
func (as *AdvancedSampler) adjustSamplingRates() {
	// Get system metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Calculate memory usage percentage
	memoryUsagePercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100
	
	// Get current system load (simplified - in production, you'd use more sophisticated metrics)
	numGoroutines := float64(runtime.NumGoroutine())
	systemLoadApprox := numGoroutines / float64(runtime.NumCPU()) / 100.0 // Rough approximation
	
	as.mu.Lock()
	defer as.mu.Unlock()
	
	// Adjust adaptive strategies
	for _, strategy := range as.strategies {
		if adaptive, ok := strategy.(*AdaptiveLoadSamplingStrategy); ok {
			adaptive.AdjustRate(systemLoadApprox, memoryUsagePercent)
		}
	}
}

// GetStats returns sampling statistics
func (as *AdvancedSampler) GetStats() map[string]SamplingStrategyStats {
	as.mu.RLock()
	defer as.mu.RUnlock()
	
	stats := make(map[string]SamplingStrategyStats)
	
	// Overall stats
	stats["overall"] = as.stats
	
	// Strategy-specific stats
	for _, strategy := range as.strategies {
		stats[strategy.Name()] = strategy.GetStats()
	}
	
	return stats
}

// FixedRateSamplingStrategy implements simple fixed-rate sampling
type FixedRateSamplingStrategy struct {
	rate  float64
	stats SamplingStrategyStats
	mu    sync.RWMutex
}

func (f *FixedRateSamplingStrategy) Name() string {
	return "fixed_rate"
}

func (f *FixedRateSamplingStrategy) ShouldSample(level slog.Level, component string, attrs map[string]interface{}) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.stats.TotalEvaluated++
	f.stats.LastEvaluation = time.Now()
	
	// Use hash-based sampling for consistency
	hash := f.hashAttributes(level, component, attrs)
	shouldSample := hash < f.rate
	
	if shouldSample {
		f.stats.TotalSampled++
	} else {
		f.stats.TotalDropped++
	}
	
	if f.stats.TotalEvaluated > 0 {
		f.stats.SampleRate = float64(f.stats.TotalSampled) / float64(f.stats.TotalEvaluated)
	}
	
	return shouldSample
}

func (f *FixedRateSamplingStrategy) UpdateConfig(config interface{}) error {
	if rate, ok := config.(float64); ok {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.rate = rate
		f.stats.LastConfigUpdate = time.Now()
		f.stats.CurrentThreshold = rate
		return nil
	}
	return fmt.Errorf("invalid config type for fixed rate strategy")
}

func (f *FixedRateSamplingStrategy) GetStats() SamplingStrategyStats {
	f.mu.RLock()
	defer f.mu.RUnlock()
	stats := f.stats
	stats.Name = f.Name()
	return stats
}

func (f *FixedRateSamplingStrategy) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stats = SamplingStrategyStats{
		Name: f.Name(),
	}
}

func (f *FixedRateSamplingStrategy) hashAttributes(level slog.Level, component string, attrs map[string]interface{}) float64 {
	hasher := fnv.New64a()
	hasher.Write([]byte(level.String()))
	hasher.Write([]byte(component))
	
	// Include some attributes in the hash for consistency
	if traceID, ok := attrs["trace_id"]; ok {
		hasher.Write([]byte(fmt.Sprintf("%v", traceID)))
	}
	
	hash := hasher.Sum64()
	return float64(hash) / float64(^uint64(0))
}

// AdaptiveLoadSamplingStrategy implements adaptive sampling based on system load
type AdaptiveLoadSamplingStrategy struct {
	config      AdaptiveLoadConfig
	currentRate float64
	stats       SamplingStrategyStats
	lastAdjust  time.Time
	mu          sync.RWMutex
}

type AdaptiveLoadConfig struct {
	InitialRate             float64
	MinRate                 float64
	MaxRate                 float64
	SystemLoadThreshold     float64
	MemoryThresholdPercent  float64
	AdjustmentInterval      time.Duration
}

func (a *AdaptiveLoadSamplingStrategy) Name() string {
	return "adaptive_load"
}

func (a *AdaptiveLoadSamplingStrategy) ShouldSample(level slog.Level, component string, attrs map[string]interface{}) bool {
	a.mu.RLock()
	currentRate := a.currentRate
	a.mu.RUnlock()
	
	// Use the same hash-based sampling as fixed rate
	hasher := fnv.New64a()
	hasher.Write([]byte(level.String()))
	hasher.Write([]byte(component))
	if traceID, ok := attrs["trace_id"]; ok {
		hasher.Write([]byte(fmt.Sprintf("%v", traceID)))
	}
	
	hash := hasher.Sum64()
	normalized := float64(hash) / float64(^uint64(0))
	
	shouldSample := normalized < currentRate
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.stats.TotalEvaluated++
	a.stats.LastEvaluation = time.Now()
	a.stats.CurrentThreshold = currentRate
	
	if shouldSample {
		a.stats.TotalSampled++
	} else {
		a.stats.TotalDropped++
	}
	
	if a.stats.TotalEvaluated > 0 {
		a.stats.SampleRate = float64(a.stats.TotalSampled) / float64(a.stats.TotalEvaluated)
	}
	
	return shouldSample
}

func (a *AdaptiveLoadSamplingStrategy) AdjustRate(systemLoad, memoryUsagePercent float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Skip if we've adjusted too recently
	if time.Since(a.lastAdjust) < a.config.AdjustmentInterval {
		return
	}
	
	oldRate := a.currentRate
	
	// Reduce sampling rate if system is under stress
	if systemLoad > a.config.SystemLoadThreshold || memoryUsagePercent > a.config.MemoryThresholdPercent {
		// Reduce rate by 20%
		a.currentRate = math.Max(a.currentRate*0.8, a.config.MinRate)
	} else {
		// Gradually increase rate back to normal
		a.currentRate = math.Min(a.currentRate*1.1, a.config.MaxRate)
	}
	
	if a.currentRate != oldRate {
		a.lastAdjust = time.Now()
		a.stats.LastConfigUpdate = time.Now()
	}
}

func (a *AdaptiveLoadSamplingStrategy) UpdateConfig(config interface{}) error {
	if cfg, ok := config.(AdaptiveLoadConfig); ok {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.config = cfg
		a.stats.LastConfigUpdate = time.Now()
		return nil
	}
	return fmt.Errorf("invalid config type for adaptive load strategy")
}

func (a *AdaptiveLoadSamplingStrategy) GetStats() SamplingStrategyStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	stats := a.stats
	stats.Name = a.Name()
	return stats
}

func (a *AdaptiveLoadSamplingStrategy) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.currentRate = a.config.InitialRate
	a.stats = SamplingStrategyStats{
		Name: a.Name(),
	}
}

// PriorityBasedSamplingStrategy implements priority-based sampling with rules
type PriorityBasedSamplingStrategy struct {
	rules []PrioritySamplingRule
	stats SamplingStrategyStats
	mu    sync.RWMutex
}

func (p *PriorityBasedSamplingStrategy) Name() string {
	return "priority_based"
}

func (p *PriorityBasedSamplingStrategy) ShouldSample(level slog.Level, component string, attrs map[string]interface{}) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.stats.TotalEvaluated++
	p.stats.LastEvaluation = time.Now()
	
	// Check rules in priority order (higher priority first)
	for _, rule := range p.rules {
		if p.matchesRule(rule, level, component, attrs) {
			shouldSample := rule.AlwaysSample || p.shouldSampleByRate(rule.SampleRate, attrs)
			
			if shouldSample {
				p.stats.TotalSampled++
			} else {
				p.stats.TotalDropped++
			}
			
			if p.stats.TotalEvaluated > 0 {
				p.stats.SampleRate = float64(p.stats.TotalSampled) / float64(p.stats.TotalEvaluated)
			}
			
			return shouldSample
		}
	}
	
	// Default to not sampling if no rules match
	p.stats.TotalDropped++
	if p.stats.TotalEvaluated > 0 {
		p.stats.SampleRate = float64(p.stats.TotalSampled) / float64(p.stats.TotalEvaluated)
	}
	
	return false
}

func (p *PriorityBasedSamplingStrategy) matchesRule(rule PrioritySamplingRule, level slog.Level, component string, attrs map[string]interface{}) bool {
	for _, condition := range rule.Conditions {
		if !p.evaluateCondition(condition, level, component, attrs) {
			return false
		}
	}
	return true
}

func (p *PriorityBasedSamplingStrategy) evaluateCondition(condition SamplingCondition, level slog.Level, component string, attrs map[string]interface{}) bool {
	var fieldValue interface{}
	
	switch condition.Field {
	case "level":
		fieldValue = level.String()
	case "component":
		fieldValue = component
	default:
		fieldValue = attrs[condition.Field]
	}
	
	return p.compareValues(fieldValue, condition.Operator, condition.Value)
}

func (p *PriorityBasedSamplingStrategy) compareValues(fieldValue interface{}, operator string, expectedValue interface{}) bool {
	switch operator {
	case "eq":
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", expectedValue)
	case "ne":
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", expectedValue)
	case "contains":
		fieldStr := fmt.Sprintf("%v", fieldValue)
		expectedStr := fmt.Sprintf("%v", expectedValue)
		return strings.Contains(fieldStr, expectedStr)
	case "gt", "lt", "gte", "lte":
		// For numeric comparisons, try to convert to float64
		fieldNum, ok1 := p.toFloat64(fieldValue)
		expectedNum, ok2 := p.toFloat64(expectedValue)
		if !ok1 || !ok2 {
			return false
		}
		
		switch operator {
		case "gt":
			return fieldNum > expectedNum
		case "lt":
			return fieldNum < expectedNum
		case "gte":
			return fieldNum >= expectedNum
		case "lte":
			return fieldNum <= expectedNum
		}
	}
	
	return false
}

func (p *PriorityBasedSamplingStrategy) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func (p *PriorityBasedSamplingStrategy) shouldSampleByRate(rate float64, attrs map[string]interface{}) bool {
	// Use trace ID for consistent sampling decisions
	hasher := fnv.New64a()
	if traceID, ok := attrs["trace_id"]; ok {
		hasher.Write([]byte(fmt.Sprintf("%v", traceID)))
	} else {
		hasher.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	
	hash := hasher.Sum64()
	normalized := float64(hash) / float64(^uint64(0))
	
	return normalized < rate
}

func (p *PriorityBasedSamplingStrategy) UpdateConfig(config interface{}) error {
	if rules, ok := config.([]PrioritySamplingRule); ok {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.rules = rules
		p.stats.LastConfigUpdate = time.Now()
		return nil
	}
	return fmt.Errorf("invalid config type for priority-based strategy")
}

func (p *PriorityBasedSamplingStrategy) GetStats() SamplingStrategyStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stats := p.stats
	stats.Name = p.Name()
	return stats
}

func (p *PriorityBasedSamplingStrategy) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = SamplingStrategyStats{
		Name: p.Name(),
	}
}

// TailBasedSamplingStrategy implements tail-based sampling
type TailBasedSamplingStrategy struct {
	config      TailBasedSamplingConfig
	traceBuffer map[string]*TraceBuffer
	stats       SamplingStrategyStats
	mu          sync.RWMutex
}

type TraceBuffer struct {
	TraceID    string
	Spans      []interface{} // Changed from []*SpanInfo to avoid circular dependency
	StartTime  time.Time
	HasError   bool
	Duration   time.Duration
	Sampled    bool
}

func NewTailBasedSamplingStrategy(config TailBasedSamplingConfig) *TailBasedSamplingStrategy {
	strategy := &TailBasedSamplingStrategy{
		config:      config,
		traceBuffer: make(map[string]*TraceBuffer),
	}
	
	// Start cleanup goroutine
	go strategy.cleanupExpiredTraces()
	
	return strategy
}

func (t *TailBasedSamplingStrategy) Name() string {
	return "tail_based"
}

func (t *TailBasedSamplingStrategy) ShouldSample(level slog.Level, component string, attrs map[string]interface{}) bool {
	// For tail-based sampling, we initially buffer everything and make decisions later
	// This is a simplified implementation - in practice, you'd integrate with trace spans
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.stats.TotalEvaluated++
	t.stats.LastEvaluation = time.Now()
	
	// For now, apply default sampling
	shouldSample := t.applyDefaultSampling(level, attrs)
	
	if shouldSample {
		t.stats.TotalSampled++
	} else {
		t.stats.TotalDropped++
	}
	
	if t.stats.TotalEvaluated > 0 {
		t.stats.SampleRate = float64(t.stats.TotalSampled) / float64(t.stats.TotalEvaluated)
	}
	
	return shouldSample
}

func (t *TailBasedSamplingStrategy) applyDefaultSampling(level slog.Level, attrs map[string]interface{}) bool {
	// Sample all errors
	if level >= slog.LevelError {
		return true
	}
	
	// Check if this is part of a slow trace
	if duration, ok := attrs["duration_ms"]; ok {
		if durationMs, ok := duration.(int64); ok && durationMs > t.config.SlowTraceThresholdMs {
			hasher := fnv.New64a()
			if traceID, ok := attrs["trace_id"]; ok {
				hasher.Write([]byte(fmt.Sprintf("%v", traceID)))
			}
			hash := hasher.Sum64()
			normalized := float64(hash) / float64(^uint64(0))
			return normalized < t.config.SlowTraceSampleRate
		}
	}
	
	// Apply default sampling rate
	hasher := fnv.New64a()
	if traceID, ok := attrs["trace_id"]; ok {
		hasher.Write([]byte(fmt.Sprintf("%v", traceID)))
	} else {
		hasher.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	hash := hasher.Sum64()
	normalized := float64(hash) / float64(^uint64(0))
	
	return normalized < t.config.DefaultSampleRate
}

func (t *TailBasedSamplingStrategy) cleanupExpiredTraces() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		t.mu.Lock()
		timeout := time.Duration(t.config.TraceTimeoutSeconds) * time.Second
		cutoff := time.Now().Add(-timeout)
		
		for traceID, buffer := range t.traceBuffer {
			if buffer.StartTime.Before(cutoff) {
				delete(t.traceBuffer, traceID)
			}
		}
		t.mu.Unlock()
	}
}

func (t *TailBasedSamplingStrategy) UpdateConfig(config interface{}) error {
	if cfg, ok := config.(TailBasedSamplingConfig); ok {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.config = cfg
		t.stats.LastConfigUpdate = time.Now()
		return nil
	}
	return fmt.Errorf("invalid config type for tail-based strategy")
}

func (t *TailBasedSamplingStrategy) GetStats() SamplingStrategyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	stats := t.stats
	stats.Name = t.Name()
	return stats
}

func (t *TailBasedSamplingStrategy) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.traceBuffer = make(map[string]*TraceBuffer)
	t.stats = SamplingStrategyStats{
		Name: t.Name(),
	}
}

// DefaultAdvancedSamplingConfig returns a default advanced sampling configuration
func DefaultAdvancedSamplingConfig() AdvancedSamplingConfig {
	return AdvancedSamplingConfig{
		Enabled:                 true,
		DefaultStrategy:         "fixed_rate",
		SystemLoadThreshold:     0.8,
		MemoryThresholdPercent:  85.0,
		AdaptiveEnabled:         true,
		AdaptiveInterval:        30,
		PriorityRules: []PrioritySamplingRule{
			{
				Name:        "always_sample_errors",
				Conditions: []SamplingCondition{
					{Field: "level", Operator: "eq", Value: "ERROR"},
				},
				AlwaysSample: true,
				Priority:     1,
			},
			{
				Name:        "sample_slow_requests",
				Conditions: []SamplingCondition{
					{Field: "duration_ms", Operator: "gt", Value: 1000},
				},
				SampleRate: 0.5,
				Priority:   2,
			},
		},
		TailBasedConfig: TailBasedSamplingConfig{
			Enabled:              false,
			BufferSize:           1000,
			TraceTimeoutSeconds:  300,
			ErrorSampleRate:      1.0,
			SlowTraceSampleRate:  0.5,
			SlowTraceThresholdMs: 1000,
			DefaultSampleRate:    0.1,
		},
	}
}