package logging

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LevelHandler provides per-component log level filtering
type LevelHandler struct {
	handler        slog.Handler
	defaultLevel   slog.Level
	componentLevels map[string]slog.Level
	mu             sync.RWMutex
}

// ComponentLevelConfig defines log levels for different components
type ComponentLevelConfig struct {
	DefaultLevel    slog.Level            `yaml:"defaultLevel" json:"defaultLevel"`
	ComponentLevels map[string]slog.Level `yaml:"componentLevels" json:"componentLevels"`
	Enabled         bool                  `yaml:"enabled" json:"enabled"`
}

// NewLevelHandler creates a new level handler with per-component filtering
func NewLevelHandler(handler slog.Handler, defaultLevel slog.Level) *LevelHandler {
	return &LevelHandler{
		handler:         handler,
		defaultLevel:    defaultLevel,
		componentLevels: make(map[string]slog.Level),
	}
}

// NewLevelHandlerWithConfig creates a new level handler with configuration
func NewLevelHandlerWithConfig(handler slog.Handler, config ComponentLevelConfig) *LevelHandler {
	lh := &LevelHandler{
		handler:         handler,
		defaultLevel:    config.DefaultLevel,
		componentLevels: make(map[string]slog.Level),
	}

	// Copy component levels
	for component, level := range config.ComponentLevels {
		lh.componentLevels[component] = level
	}

	return lh
}

// Handle implements slog.Handler interface with per-component level filtering
func (lh *LevelHandler) Handle(ctx context.Context, record slog.Record) error {
	component := lh.extractComponent(ctx, record)
	requiredLevel := lh.getLevelForComponent(component)

	// Check if the record level meets the required level for this component
	if record.Level < requiredLevel {
		return nil // Skip this log entry
	}

	return lh.handler.Handle(ctx, record)
}

// WithAttrs implements slog.Handler interface
func (lh *LevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LevelHandler{
		handler:         lh.handler.WithAttrs(attrs),
		defaultLevel:    lh.defaultLevel,
		componentLevels: lh.copyComponentLevels(),
	}
}

// WithGroup implements slog.Handler interface
func (lh *LevelHandler) WithGroup(name string) slog.Handler {
	return &LevelHandler{
		handler:         lh.handler.WithGroup(name),
		defaultLevel:    lh.defaultLevel,
		componentLevels: lh.copyComponentLevels(),
	}
}

// Enabled implements slog.Handler interface
func (lh *LevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Extract component from context
	component := lh.extractComponent(ctx, slog.Record{})
	requiredLevel := lh.getLevelForComponent(component)

	// Check if level is enabled for this component
	if level < requiredLevel {
		return false
	}

	return lh.handler.Enabled(ctx, level)
}

// extractComponent extracts the component name from context or record attributes
func (lh *LevelHandler) extractComponent(ctx context.Context, record slog.Record) string {
	// First, try to get component from context
	if component := GetComponent(ctx); component != "" {
		return component
	}

	// Then, try to extract from record attributes
	var componentName string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "component" {
			componentName = attr.Value.String()
			return false // Stop iteration
		}
		return true // Continue iteration
	})

	if componentName != "" {
		return componentName
	}

	// Finally, try to extract from groups (if the handler was created with WithGroup)
	// This is a simplified approach - in practice, you might want to track groups
	return "default"
}

// getLevelForComponent returns the log level for a specific component
func (lh *LevelHandler) getLevelForComponent(component string) slog.Level {
	lh.mu.RLock()
	defer lh.mu.RUnlock()

	if level, exists := lh.componentLevels[component]; exists {
		return level
	}

	// Check for wildcard patterns (simple prefix matching)
	for pattern, level := range lh.componentLevels {
		if strings.Contains(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(component, prefix) {
				return level
			}
		}
	}

	return lh.defaultLevel
}

// SetComponentLevel sets the log level for a specific component
func (lh *LevelHandler) SetComponentLevel(component string, level slog.Level) {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	
	lh.componentLevels[component] = level
}

// RemoveComponentLevel removes the specific level for a component, reverting to default
func (lh *LevelHandler) RemoveComponentLevel(component string) {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	
	delete(lh.componentLevels, component)
}

// GetComponentLevel returns the log level for a specific component
func (lh *LevelHandler) GetComponentLevel(component string) slog.Level {
	return lh.getLevelForComponent(component)
}

// GetAllComponentLevels returns all component-specific levels
func (lh *LevelHandler) GetAllComponentLevels() map[string]slog.Level {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	
	return lh.copyComponentLevels()
}

// copyComponentLevels creates a copy of the component levels map
func (lh *LevelHandler) copyComponentLevels() map[string]slog.Level {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	
	copy := make(map[string]slog.Level)
	for component, level := range lh.componentLevels {
		copy[component] = level
	}
	return copy
}

// SetDefaultLevel sets the default log level
func (lh *LevelHandler) SetDefaultLevel(level slog.Level) {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	
	lh.defaultLevel = level
}

// GetDefaultLevel returns the default log level
func (lh *LevelHandler) GetDefaultLevel() slog.Level {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	
	return lh.defaultLevel
}

// UpdateConfig updates the level handler configuration
func (lh *LevelHandler) UpdateConfig(config ComponentLevelConfig) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	lh.defaultLevel = config.DefaultLevel
	
	// Clear existing component levels
	lh.componentLevels = make(map[string]slog.Level)
	
	// Set new component levels
	for component, level := range config.ComponentLevels {
		lh.componentLevels[component] = level
	}
}

// GetConfig returns the current configuration
func (lh *LevelHandler) GetConfig() ComponentLevelConfig {
	lh.mu.RLock()
	defer lh.mu.RUnlock()

	return ComponentLevelConfig{
		DefaultLevel:    lh.defaultLevel,
		ComponentLevels: lh.copyComponentLevels(),
		Enabled:         true,
	}
}

// HierarchicalLevelHandler provides hierarchical component-based level filtering
type HierarchicalLevelHandler struct {
	*LevelHandler
	hierarchy map[string][]string // component -> parent chain
}

// NewHierarchicalLevelHandler creates a level handler with hierarchical component support
func NewHierarchicalLevelHandler(handler slog.Handler, defaultLevel slog.Level) *HierarchicalLevelHandler {
	return &HierarchicalLevelHandler{
		LevelHandler: NewLevelHandler(handler, defaultLevel),
		hierarchy:    make(map[string][]string),
	}
}

// SetComponentHierarchy defines the parent chain for a component
func (hlh *HierarchicalLevelHandler) SetComponentHierarchy(component string, parents []string) {
	hlh.mu.Lock()
	defer hlh.mu.Unlock()
	
	hlh.hierarchy[component] = make([]string, len(parents))
	copy(hlh.hierarchy[component], parents)
}

// getLevelForComponent returns the log level for a component, checking parent hierarchy
func (hlh *HierarchicalLevelHandler) getLevelForComponent(component string) slog.Level {
	hlh.mu.RLock()
	defer hlh.mu.RUnlock()

	// Check direct component level
	if level, exists := hlh.componentLevels[component]; exists {
		return level
	}

	// Check parent hierarchy
	if parents, exists := hlh.hierarchy[component]; exists {
		for _, parent := range parents {
			if level, exists := hlh.componentLevels[parent]; exists {
				return level
			}
		}
	}

	// Check for wildcard patterns
	for pattern, level := range hlh.componentLevels {
		if strings.Contains(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(component, prefix) {
				return level
			}
		}
	}

	return hlh.defaultLevel
}

// Handle overrides the base Handle to use hierarchical level resolution
func (hlh *HierarchicalLevelHandler) Handle(ctx context.Context, record slog.Record) error {
	component := hlh.extractComponent(ctx, record)
	requiredLevel := hlh.getLevelForComponent(component)

	if record.Level < requiredLevel {
		return nil
	}

	return hlh.handler.Handle(ctx, record)
}

// DynamicLevelHandler provides runtime-adjustable log levels
type DynamicLevelHandler struct {
	*LevelHandler
	levelAdjusters []LevelAdjuster
	adjustMu       sync.RWMutex
}

// LevelAdjuster defines an interface for dynamic level adjustment
type LevelAdjuster interface {
	AdjustLevel(component string, originalLevel slog.Level) slog.Level
	Name() string
}

// NewDynamicLevelHandler creates a handler with dynamic level adjustment
func NewDynamicLevelHandler(handler slog.Handler, defaultLevel slog.Level) *DynamicLevelHandler {
	return &DynamicLevelHandler{
		LevelHandler: NewLevelHandler(handler, defaultLevel),
	}
}

// AddLevelAdjuster adds a dynamic level adjuster
func (dlh *DynamicLevelHandler) AddLevelAdjuster(adjuster LevelAdjuster) {
	dlh.adjustMu.Lock()
	defer dlh.adjustMu.Unlock()
	
	dlh.levelAdjusters = append(dlh.levelAdjusters, adjuster)
}

// RemoveLevelAdjuster removes a level adjuster by name
func (dlh *DynamicLevelHandler) RemoveLevelAdjuster(name string) {
	dlh.adjustMu.Lock()
	defer dlh.adjustMu.Unlock()

	for i, adjuster := range dlh.levelAdjusters {
		if adjuster.Name() == name {
			dlh.levelAdjusters = append(dlh.levelAdjusters[:i], dlh.levelAdjusters[i+1:]...)
			break
		}
	}
}

// getLevelForComponent applies dynamic adjustments to the component level
func (dlh *DynamicLevelHandler) getLevelForComponent(component string) slog.Level {
	baseLevel := dlh.LevelHandler.getLevelForComponent(component)
	
	dlh.adjustMu.RLock()
	adjusters := make([]LevelAdjuster, len(dlh.levelAdjusters))
	copy(adjusters, dlh.levelAdjusters)
	dlh.adjustMu.RUnlock()

	adjustedLevel := baseLevel
	for _, adjuster := range adjusters {
		adjustedLevel = adjuster.AdjustLevel(component, adjustedLevel)
	}

	return adjustedLevel
}

// Handle overrides to use dynamic level adjustment
func (dlh *DynamicLevelHandler) Handle(ctx context.Context, record slog.Record) error {
	component := dlh.extractComponent(ctx, record)
	requiredLevel := dlh.getLevelForComponent(component)

	if record.Level < requiredLevel {
		return nil
	}

	return dlh.handler.Handle(ctx, record)
}

// Simple level adjusters

// LoadBasedLevelAdjuster adjusts log level based on system load
type LoadBasedLevelAdjuster struct {
	name         string
	getSystemLoad func() float64
	highLoadThreshold float64
	lowLoadThreshold  float64
}

// NewLoadBasedLevelAdjuster creates a load-based level adjuster
func NewLoadBasedLevelAdjuster(getSystemLoad func() float64, highLoadThreshold, lowLoadThreshold float64) *LoadBasedLevelAdjuster {
	return &LoadBasedLevelAdjuster{
		name:              "load_based",
		getSystemLoad:     getSystemLoad,
		highLoadThreshold: highLoadThreshold,
		lowLoadThreshold:  lowLoadThreshold,
	}
}

// AdjustLevel adjusts the log level based on system load
func (lba *LoadBasedLevelAdjuster) AdjustLevel(component string, originalLevel slog.Level) slog.Level {
	if lba.getSystemLoad == nil {
		return originalLevel
	}

	load := lba.getSystemLoad()
	
	if load > lba.highLoadThreshold {
		// High load - increase level (reduce logging)
		if originalLevel < slog.LevelWarn {
			return slog.LevelWarn
		}
	} else if load < lba.lowLoadThreshold {
		// Low load - decrease level (increase logging)
		if originalLevel > slog.LevelDebug {
			return slog.LevelDebug
		}
	}

	return originalLevel
}

// Name returns the adjuster name
func (lba *LoadBasedLevelAdjuster) Name() string {
	return lba.name
}

// TimeBasedLevelAdjuster adjusts log level based on time of day
type TimeBasedLevelAdjuster struct {
	name           string
	businessHours  []TimeRange
	businessLevel  slog.Level
	offHoursLevel  slog.Level
}

// TimeRange represents a time range
type TimeRange struct {
	Start int // Hour of day (0-23)
	End   int // Hour of day (0-23)
}

// NewTimeBasedLevelAdjuster creates a time-based level adjuster
func NewTimeBasedLevelAdjuster(businessHours []TimeRange, businessLevel, offHoursLevel slog.Level) *TimeBasedLevelAdjuster {
	return &TimeBasedLevelAdjuster{
		name:          "time_based",
		businessHours: businessHours,
		businessLevel: businessLevel,
		offHoursLevel: offHoursLevel,
	}
}

// AdjustLevel adjusts level based on current time
func (tba *TimeBasedLevelAdjuster) AdjustLevel(component string, originalLevel slog.Level) slog.Level {
	currentHour := time.Now().Hour()
	
	// Check if current time is within business hours
	for _, timeRange := range tba.businessHours {
		if currentHour >= timeRange.Start && currentHour < timeRange.End {
			return tba.businessLevel
		}
	}

	return tba.offHoursLevel
}

// Name returns the adjuster name
func (tba *TimeBasedLevelAdjuster) Name() string {
	return tba.name
}

// ComponentPatternLevelAdjuster adjusts levels based on component name patterns
type ComponentPatternLevelAdjuster struct {
	name     string
	patterns map[string]slog.Level // pattern -> level
}

// NewComponentPatternLevelAdjuster creates a pattern-based adjuster
func NewComponentPatternLevelAdjuster(patterns map[string]slog.Level) *ComponentPatternLevelAdjuster {
	return &ComponentPatternLevelAdjuster{
		name:     "pattern_based",
		patterns: patterns,
	}
}

// AdjustLevel adjusts level based on component name patterns
func (cpla *ComponentPatternLevelAdjuster) AdjustLevel(component string, originalLevel slog.Level) slog.Level {
	for pattern, level := range cpla.patterns {
		if matched, _ := filepath.Match(pattern, component); matched {
			return level
		}
	}

	return originalLevel
}

// Name returns the adjuster name
func (cpla *ComponentPatternLevelAdjuster) Name() string {
	return cpla.name
}

// LevelHandlerStats provides statistics about level filtering
type LevelHandlerStats struct {
	ComponentLevels  map[string]slog.Level `json:"component_levels"`
	DefaultLevel     slog.Level            `json:"default_level"`
	ActiveComponents []string              `json:"active_components"`
	TotalComponents  int                   `json:"total_components"`
}

// GetStats returns level handler statistics
func (lh *LevelHandler) GetStats() LevelHandlerStats {
	lh.mu.RLock()
	defer lh.mu.RUnlock()

	activeComponents := make([]string, 0, len(lh.componentLevels))
	for component := range lh.componentLevels {
		activeComponents = append(activeComponents, component)
	}

	return LevelHandlerStats{
		ComponentLevels:  lh.copyComponentLevels(),
		DefaultLevel:     lh.defaultLevel,
		ActiveComponents: activeComponents,
		TotalComponents:  len(lh.componentLevels),
	}
}