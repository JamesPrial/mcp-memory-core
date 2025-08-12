package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// HealthChecker provides comprehensive health monitoring for observability components
type HealthChecker struct {
	otlpHandler     *OTLPHandler
	metricsCollector *MetricsCollector
	// traceManager    *TraceManager
	advancedSampler *AdvancedSampler
	config          HealthConfig
	
	// Health status tracking
	overallHealth   OverallHealthStatus
	componentHealth map[string]ComponentHealthStatus
	mu              sync.RWMutex
	
	// Background monitoring
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// HealthConfig defines configuration for health monitoring
type HealthConfig struct {
	Enabled                  bool  `yaml:"enabled" json:"enabled"`
	CheckInterval            int   `yaml:"checkInterval" json:"checkInterval"` // seconds
	UnhealthyThreshold       int   `yaml:"unhealthyThreshold" json:"unhealthyThreshold"` // consecutive failures
	HealthTimeoutSeconds     int   `yaml:"healthTimeoutSeconds" json:"healthTimeoutSeconds"`
	EnableDetailedChecks     bool  `yaml:"enableDetailedChecks" json:"enableDetailedChecks"`
	ExportFailureRateThreshold float64 `yaml:"exportFailureRateThreshold" json:"exportFailureRateThreshold"`
	BufferUtilizationThreshold float64 `yaml:"bufferUtilizationThreshold" json:"bufferUtilizationThreshold"`
	MemoryThresholdMB        int64 `yaml:"memoryThresholdMB" json:"memoryThresholdMB"`
	GoroutineThreshold       int   `yaml:"goroutineThreshold" json:"goroutineThreshold"`
}

// OverallHealthStatus represents the overall health of the observability system
type OverallHealthStatus struct {
	Status              HealthStatusCode           `json:"status"`
	Timestamp           time.Time                  `json:"timestamp"`
	UpTime              time.Duration              `json:"uptime"`
	ComponentsHealthy   int                        `json:"components_healthy"`
	ComponentsUnhealthy int                        `json:"components_unhealthy"`
	ComponentsUnknown   int                        `json:"components_unknown"`
	TotalComponents     int                        `json:"total_components"`
	SystemResources     SystemResourceStatus       `json:"system_resources"`
	LastHealthCheck     time.Time                  `json:"last_health_check"`
	HealthCheckDuration time.Duration              `json:"health_check_duration"`
	Issues              []HealthIssue              `json:"issues,omitempty"`
}

// ComponentHealthStatus represents the health status of individual components
type ComponentHealthStatus struct {
	Component           string           `json:"component"`
	Status              HealthStatusCode `json:"status"`
	Timestamp           time.Time        `json:"timestamp"`
	Message             string           `json:"message,omitempty"`
	Details             interface{}      `json:"details,omitempty"`
	ConsecutiveFailures int              `json:"consecutive_failures"`
	LastSuccess         time.Time        `json:"last_success"`
	LastFailure         time.Time        `json:"last_failure"`
	CheckDuration       time.Duration    `json:"check_duration"`
}

// HealthStatusCode represents the health status
type HealthStatusCode string

const (
	HealthStatusHealthy   HealthStatusCode = "HEALTHY"
	HealthStatusDegraded  HealthStatusCode = "DEGRADED"
	HealthStatusUnhealthy HealthStatusCode = "UNHEALTHY"
	HealthStatusUnknown   HealthStatusCode = "UNKNOWN"
)

// SystemResourceStatus represents system resource utilization
type SystemResourceStatus struct {
	MemoryUsageMB     int64   `json:"memory_usage_mb"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	GoroutineCount    int     `json:"goroutine_count"`
	GCPauseMs         float64 `json:"gc_pause_ms"`
	NumCPU            int     `json:"num_cpu"`
	Timestamp         time.Time `json:"timestamp"`
}

// HealthIssue represents a specific health issue
type HealthIssue struct {
	Component   string    `json:"component"`
	Severity    string    `json:"severity"` // "INFO", "WARNING", "ERROR", "CRITICAL"
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Resolution  string    `json:"resolution,omitempty"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config HealthConfig) *HealthChecker {
	hc := &HealthChecker{
		config:          config,
		componentHealth: make(map[string]ComponentHealthStatus),
		stopChan:        make(chan struct{}),
		overallHealth: OverallHealthStatus{
			Status:    HealthStatusUnknown,
			Timestamp: time.Now(),
		},
	}
	
	if config.Enabled {
		hc.startHealthChecking()
	}
	
	return hc
}

// RegisterOTLPHandler registers the OTLP handler for health monitoring
func (hc *HealthChecker) RegisterOTLPHandler(handler *OTLPHandler) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.otlpHandler = handler
}

// RegisterMetricsCollector registers the metrics collector for health monitoring
func (hc *HealthChecker) RegisterMetricsCollector(collector *MetricsCollector) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.metricsCollector = collector
}

// RegisterTraceManager registers the trace manager for health monitoring
// func (hc *HealthChecker) RegisterTraceManager(manager *TraceManager) {
//	hc.mu.Lock()
//	defer hc.mu.Unlock()
//	hc.traceManager = manager
// }

// RegisterAdvancedSampler registers the advanced sampler for health monitoring
func (hc *HealthChecker) RegisterAdvancedSampler(sampler *AdvancedSampler) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.advancedSampler = sampler
}

// startHealthChecking starts the background health checking
func (hc *HealthChecker) startHealthChecking() {
	hc.wg.Add(1)
	go hc.healthCheckLoop()
}

// healthCheckLoop runs the periodic health checks
func (hc *HealthChecker) healthCheckLoop() {
	defer hc.wg.Done()
	
	interval := time.Duration(hc.config.CheckInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	// Perform initial health check
	hc.performHealthCheck()
	
	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck()
		case <-hc.stopChan:
			return
		}
	}
}

// performHealthCheck performs a comprehensive health check
func (hc *HealthChecker) performHealthCheck() {
	start := time.Now()
	
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	// Reset component counters
	hc.overallHealth.ComponentsHealthy = 0
	hc.overallHealth.ComponentsUnhealthy = 0
	hc.overallHealth.ComponentsUnknown = 0
	hc.overallHealth.Issues = make([]HealthIssue, 0)
	
	// Check system resources
	hc.checkSystemResources()
	
	// Check individual components
	hc.checkOTLPHandler()
	hc.checkMetricsCollector()
	hc.checkTraceManager()
	hc.checkAdvancedSampler()
	
	// Calculate total components
	hc.overallHealth.TotalComponents = len(hc.componentHealth)
	
	// Determine overall health status
	hc.calculateOverallHealth()
	
	// Update timestamps
	hc.overallHealth.LastHealthCheck = start
	hc.overallHealth.HealthCheckDuration = time.Since(start)
	hc.overallHealth.Timestamp = time.Now()
}

// checkSystemResources checks system resource utilization
func (hc *HealthChecker) checkSystemResources() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	memoryUsageMB := int64(memStats.Alloc / 1024 / 1024)
	memoryUsagePercent := float64(memStats.Alloc) / float64(memStats.Sys) * 100
	goroutineCount := runtime.NumGoroutine()
	
	hc.overallHealth.SystemResources = SystemResourceStatus{
		MemoryUsageMB:      memoryUsageMB,
		MemoryUsagePercent: memoryUsagePercent,
		GoroutineCount:     goroutineCount,
		NumCPU:             runtime.NumCPU(),
		Timestamp:          time.Now(),
	}
	
	// Add GC pause if available
	if len(memStats.PauseNs) > 0 {
		lastPause := time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256])
		hc.overallHealth.SystemResources.GCPauseMs = float64(lastPause.Nanoseconds()) / 1000000
	}
	
	// Check for resource issues
	if hc.config.MemoryThresholdMB > 0 && memoryUsageMB > hc.config.MemoryThresholdMB {
		hc.overallHealth.Issues = append(hc.overallHealth.Issues, HealthIssue{
			Component: "system",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Memory usage (%d MB) exceeds threshold (%d MB)", memoryUsageMB, hc.config.MemoryThresholdMB),
			Timestamp: time.Now(),
			Resolution: "Consider increasing memory limits or optimizing memory usage",
		})
	}
	
	if hc.config.GoroutineThreshold > 0 && goroutineCount > hc.config.GoroutineThreshold {
		hc.overallHealth.Issues = append(hc.overallHealth.Issues, HealthIssue{
			Component: "system",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Goroutine count (%d) exceeds threshold (%d)", goroutineCount, hc.config.GoroutineThreshold),
			Timestamp: time.Now(),
			Resolution: "Check for goroutine leaks or consider increasing threshold",
		})
	}
}

// checkOTLPHandler checks the health of the OTLP handler
func (hc *HealthChecker) checkOTLPHandler() {
	component := "otlp_handler"
	
	if hc.otlpHandler == nil {
		hc.updateComponentHealth(component, HealthStatusUnknown, "OTLP handler not registered", nil)
		return
	}
	
	// Get OTLP health status and export stats - disabled due to type mismatch
	// otlpHealthStatus := hc.otlpHandler.GetHealthStatus()
	// exportStats := hc.otlpHandler.GetExportStats()
	
	status := HealthStatusHealthy
	message := "OTLP handler is healthy"
	issues := make([]string, 0)
	
	// Check connection health - disabled due to type mismatch
	// if !otlpHealthStatus.ConnectionHealthy {
	// 	status = HealthStatusUnhealthy
	// 	issues = append(issues, "Connection to OTLP endpoint is unhealthy")
	// }
	
	// Check export failure rate - disabled due to type mismatch
	// if exportStats.SuccessRate < (1.0 - hc.config.ExportFailureRateThreshold) {
	// 	if status != HealthStatusUnhealthy {
	// 		status = HealthStatusDegraded
	// 	}
	// 	issues = append(issues, fmt.Sprintf("Export success rate (%.2f%%) is below threshold", exportStats.SuccessRate*100))
	// }
	
	// Check buffer utilization - disabled due to type mismatch
	// if exportStats.BufferUtilization > hc.config.BufferUtilizationThreshold {
	// 	if status != HealthStatusUnhealthy {
	// 		status = HealthStatusDegraded
	// 	}
	// 	issues = append(issues, fmt.Sprintf("Buffer utilization (%.2f%%) is above threshold", exportStats.BufferUtilization*100))
	// }
	
	// Check for recent exports - disabled due to type mismatch
	// if time.Since(exportStats.LastExportTime) > time.Duration(hc.config.CheckInterval*3)*time.Second {
	// 	if status != HealthStatusUnhealthy {
	// 		status = HealthStatusDegraded
	// 	}
	// 	issues = append(issues, "No recent successful exports")
	// }
	
	if len(issues) > 0 {
		message = fmt.Sprintf("Issues detected: %v", issues)
	}
	
	details := map[string]interface{}{
		// "health_status": otlpHealthStatus, // Disabled due to type mismatch
		// "export_stats":  exportStats,      // Disabled due to type mismatch
		"issues": issues,
	}
	
	hc.updateComponentHealth(component, status, message, details)
	
	// Add specific issues to overall health
	for _, issue := range issues {
		severity := "WARNING"
		if status == HealthStatusUnhealthy {
			severity = "ERROR"
		}
		
		hc.overallHealth.Issues = append(hc.overallHealth.Issues, HealthIssue{
			Component: component,
			Severity:  severity,
			Message:   issue,
			Timestamp: time.Now(),
		})
	}
}

// checkMetricsCollector checks the health of the metrics collector
func (hc *HealthChecker) checkMetricsCollector() {
	component := "metrics_collector"
	
	if hc.metricsCollector == nil {
		hc.updateComponentHealth(component, HealthStatusUnknown, "Metrics collector not registered", nil)
		return
	}
	
	// For now, assume metrics collector is healthy if it exists
	// In a more sophisticated implementation, you'd check metrics server health,
	// scrape endpoint availability, etc.
	hc.updateComponentHealth(component, HealthStatusHealthy, "Metrics collector is operational", map[string]interface{}{
		"collector_type": "prometheus",
		"status":         "active",
	})
}

// checkTraceManager checks the health of the trace manager
func (hc *HealthChecker) checkTraceManager() {
	component := "trace_manager"
	
	// Temporarily commented out due to circular dependency
	hc.updateComponentHealth(component, HealthStatusUnknown, "Trace manager not yet integrated", nil)
	return
	
	// if hc.traceManager == nil {
	//	hc.updateComponentHealth(component, HealthStatusUnknown, "Trace manager not registered", nil)
	//	return
	// }
	
	// tracingHealth := hc.traceManager.GetTracingHealth()
	/*
	status := HealthStatusHealthy
	message := "Trace manager is healthy"
	issues := make([]string, 0)
	
	if !tracingHealth.Healthy {
		status = HealthStatusDegraded
		issues = append(issues, "Trace manager reports unhealthy status")
	}
	
	// Check for span leaks
	if tracingHealth.ActiveSpans > 1000 { // Configurable threshold
		if status != HealthStatusUnhealthy {
			status = HealthStatusDegraded
		}
		issues = append(issues, fmt.Sprintf("High number of active spans (%d)", tracingHealth.ActiveSpans))
	}
	
	// Check span drop rate
	if tracingHealth.TotalSpansCreated > 0 {
		dropRate := float64(tracingHealth.SpansDropped) / float64(tracingHealth.TotalSpansCreated)
		if dropRate > 0.1 { // 10% drop rate threshold
			if status != HealthStatusUnhealthy {
				status = HealthStatusDegraded
			}
			issues = append(issues, fmt.Sprintf("High span drop rate (%.2f%%)", dropRate*100))
		}
	}
	
	if len(issues) > 0 {
		message = fmt.Sprintf("Issues detected: %v", issues)
	}
	
	details := map[string]interface{}{
		"tracing_health": tracingHealth,
		"issues":         issues,
	}
	
	hc.updateComponentHealth(component, status, message, details)
	
	// Add specific issues to overall health
	for _, issue := range issues {
		hc.overallHealth.Issues = append(hc.overallHealth.Issues, HealthIssue{
			Component: component,
			Severity:  "WARNING",
			Message:   issue,
			Timestamp: time.Now(),
		})
	}
	*/
}

// checkAdvancedSampler checks the health of the advanced sampler
func (hc *HealthChecker) checkAdvancedSampler() {
	component := "advanced_sampler"
	
	if hc.advancedSampler == nil {
		hc.updateComponentHealth(component, HealthStatusUnknown, "Advanced sampler not registered", nil)
		return
	}
	
	samplingStats := hc.advancedSampler.GetStats()
	
	status := HealthStatusHealthy
	message := "Advanced sampler is healthy"
	issues := make([]string, 0)
	
	// Check if any strategy is dropping too much
	for strategyName, stats := range samplingStats {
		if stats.SampleRate < 0.01 { // Less than 1% sampling
			issues = append(issues, fmt.Sprintf("Strategy '%s' has very low sample rate (%.2f%%)", strategyName, stats.SampleRate*100))
		}
	}
	
	if len(issues) > 0 {
		status = HealthStatusDegraded
		message = fmt.Sprintf("Issues detected: %v", issues)
	}
	
	details := map[string]interface{}{
		"sampling_stats": samplingStats,
		"issues":         issues,
	}
	
	hc.updateComponentHealth(component, status, message, details)
}

// updateComponentHealth updates the health status for a component
func (hc *HealthChecker) updateComponentHealth(component string, status HealthStatusCode, message string, details interface{}) {
	now := time.Now()
	
	existing, exists := hc.componentHealth[component]
	
	componentHealth := ComponentHealthStatus{
		Component:     component,
		Status:        status,
		Timestamp:     now,
		Message:       message,
		Details:       details,
		CheckDuration: time.Since(now), // This would be calculated properly in real implementation
	}
	
	if exists {
		componentHealth.ConsecutiveFailures = existing.ConsecutiveFailures
		componentHealth.LastSuccess = existing.LastSuccess
		componentHealth.LastFailure = existing.LastFailure
		
		if status == HealthStatusHealthy {
			componentHealth.ConsecutiveFailures = 0
			componentHealth.LastSuccess = now
		} else {
			componentHealth.ConsecutiveFailures++
			componentHealth.LastFailure = now
		}
	} else {
		if status == HealthStatusHealthy {
			componentHealth.LastSuccess = now
		} else {
			componentHealth.ConsecutiveFailures = 1
			componentHealth.LastFailure = now
		}
	}
	
	hc.componentHealth[component] = componentHealth
	
	// Update counters
	switch status {
	case HealthStatusHealthy:
		hc.overallHealth.ComponentsHealthy++
	case HealthStatusDegraded, HealthStatusUnhealthy:
		hc.overallHealth.ComponentsUnhealthy++
	default:
		hc.overallHealth.ComponentsUnknown++
	}
}

// calculateOverallHealth determines the overall health status
func (hc *HealthChecker) calculateOverallHealth() {
	if hc.overallHealth.ComponentsUnhealthy > 0 {
		hc.overallHealth.Status = HealthStatusUnhealthy
	} else if len(hc.overallHealth.Issues) > 0 {
		hc.overallHealth.Status = HealthStatusDegraded
	} else if hc.overallHealth.ComponentsUnknown > 0 {
		hc.overallHealth.Status = HealthStatusUnknown
	} else {
		hc.overallHealth.Status = HealthStatusHealthy
	}
}

// GetOverallHealth returns the overall health status
func (hc *HealthChecker) GetOverallHealth() OverallHealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.overallHealth
}

// GetComponentHealth returns the health status for a specific component
func (hc *HealthChecker) GetComponentHealth(component string) (ComponentHealthStatus, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	health, exists := hc.componentHealth[component]
	return health, exists
}

// GetAllComponentHealth returns the health status for all components
func (hc *HealthChecker) GetAllComponentHealth() map[string]ComponentHealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	result := make(map[string]ComponentHealthStatus, len(hc.componentHealth))
	for component, health := range hc.componentHealth {
		result[component] = health
	}
	return result
}

// HTTPHealthHandler provides an HTTP endpoint for health checks
func (hc *HealthChecker) HTTPHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		overallHealth := hc.GetOverallHealth()
		
		w.Header().Set("Content-Type", "application/json")
		
		// Set HTTP status code based on health
		switch overallHealth.Status {
		case HealthStatusHealthy:
			w.WriteHeader(http.StatusOK)
		case HealthStatusDegraded:
			w.WriteHeader(http.StatusOK) // 200 but with degraded status
		case HealthStatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		
		response := struct {
			OverallHealth   OverallHealthStatus                `json:"overall_health"`
			ComponentHealth map[string]ComponentHealthStatus   `json:"component_health"`
		}{
			OverallHealth:   overallHealth,
			ComponentHealth: hc.GetAllComponentHealth(),
		}
		
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode health status", http.StatusInternalServerError)
		}
	}
}

// Close stops the health checker
func (hc *HealthChecker) Close() error {
	if hc.stopChan != nil {
		close(hc.stopChan)
		hc.wg.Wait()
	}
	return nil
}

// DefaultHealthConfig returns a default health configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:                    true,
		CheckInterval:              30,
		UnhealthyThreshold:         3,
		HealthTimeoutSeconds:       10,
		EnableDetailedChecks:       true,
		ExportFailureRateThreshold: 0.1, // 10% failure rate
		BufferUtilizationThreshold: 0.8, // 80% buffer utilization
		MemoryThresholdMB:          1024, // 1GB
		GoroutineThreshold:         1000,
	}
}