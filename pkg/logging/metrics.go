package logging

import (
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector provides performance metrics collection using Prometheus
type MetricsCollector struct {
	// Request metrics
	requestDuration    *prometheus.HistogramVec
	requestCounter     *prometheus.CounterVec
	activeRequests     *prometheus.GaugeVec
	requestSize        *prometheus.HistogramVec
	responseSize       *prometheus.HistogramVec

	// System metrics
	memoryUsage        prometheus.Gauge
	goroutineCount     prometheus.Gauge
	gcDuration         *prometheus.HistogramVec
	allocatedBytes     prometheus.Gauge
	systemCpuUsage     prometheus.Gauge

	// Component metrics
	componentOperations *prometheus.CounterVec
	componentDuration   *prometheus.HistogramVec
	componentErrors     *prometheus.CounterVec

	// Storage metrics
	storageQueries     *prometheus.CounterVec
	storageConnections prometheus.Gauge
	storageLatency     *prometheus.HistogramVec

	// Cache metrics
	cacheHits   *prometheus.CounterVec
	cacheMisses *prometheus.CounterVec
	cacheSize   *prometheus.GaugeVec

	// Logging metrics
	logMessages   *prometheus.CounterVec
	logErrors     *prometheus.CounterVec
	logDropped    *prometheus.CounterVec
	auditEvents   *prometheus.CounterVec

	registry   *prometheus.Registry
	gatherer   prometheus.Gatherer
	mu         sync.RWMutex
	config     MetricsConfig
}

// MetricsConfig defines configuration for metrics collection
type MetricsConfig struct {
	Enabled        bool              `yaml:"enabled" json:"enabled"`
	ListenAddr     string            `yaml:"listenAddr,omitempty" json:"listenAddr,omitempty"`
	Path           string            `yaml:"path,omitempty" json:"path,omitempty"`
	Namespace      string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Subsystem      string            `yaml:"subsystem,omitempty" json:"subsystem,omitempty"`
	Labels         map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	EnableRuntime  bool              `yaml:"enableRuntime" json:"enableRuntime"`
	CollectInterval time.Duration    `yaml:"collectInterval,omitempty" json:"collectInterval,omitempty"`
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled:        true,
		ListenAddr:     ":9090",
		Path:           "/metrics",
		Namespace:      "mcp_memory",
		Subsystem:      "core",
		EnableRuntime:  true,
		CollectInterval: 15 * time.Second,
	}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config MetricsConfig) *MetricsCollector {
	if !config.Enabled {
		return nil
	}

	registry := prometheus.NewRegistry()
	
	mc := &MetricsCollector{
		registry: registry,
		gatherer: registry,
		config:   config,
	}

	mc.initializeMetrics()
	
	// Register with custom registry
	registry.MustRegister(mc.requestDuration)
	registry.MustRegister(mc.requestCounter)
	registry.MustRegister(mc.activeRequests)
	registry.MustRegister(mc.requestSize)
	registry.MustRegister(mc.responseSize)
	registry.MustRegister(mc.memoryUsage)
	registry.MustRegister(mc.goroutineCount)
	registry.MustRegister(mc.gcDuration)
	registry.MustRegister(mc.allocatedBytes)
	registry.MustRegister(mc.systemCpuUsage)
	registry.MustRegister(mc.componentOperations)
	registry.MustRegister(mc.componentDuration)
	registry.MustRegister(mc.componentErrors)
	registry.MustRegister(mc.storageQueries)
	registry.MustRegister(mc.storageConnections)
	registry.MustRegister(mc.storageLatency)
	registry.MustRegister(mc.cacheHits)
	registry.MustRegister(mc.cacheMisses)
	registry.MustRegister(mc.cacheSize)
	registry.MustRegister(mc.logMessages)
	registry.MustRegister(mc.logErrors)
	registry.MustRegister(mc.logDropped)
	registry.MustRegister(mc.auditEvents)

	// Start runtime metrics collection if enabled
	if config.EnableRuntime {
		go mc.collectRuntimeMetrics()
	}

	return mc
}

// initializeMetrics initializes all Prometheus metrics
func (mc *MetricsCollector) initializeMetrics() {
	labels := []string{"method", "endpoint", "status_code"}
	componentLabels := []string{"component", "operation"}
	storageLabels := []string{"operation", "table"}
	cacheLabels := []string{"cache_type", "key_pattern"}
	logLabels := []string{"level", "component"}

	// Request metrics
	mc.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		labels,
	)

	mc.requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		labels,
	)

	mc.activeRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "http_requests_active",
			Help:      "Number of active HTTP requests",
		},
		[]string{"method", "endpoint"},
	)

	mc.requestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "http_request_size_bytes",
			Help:      "Size of HTTP requests in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "endpoint"},
	)

	mc.responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "http_response_size_bytes",
			Help:      "Size of HTTP responses in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		},
		labels,
	)

	// System metrics
	mc.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		},
	)

	mc.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "goroutines_total",
			Help:      "Number of goroutines",
		},
	)

	mc.gcDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "gc_duration_seconds",
			Help:      "Duration of garbage collection cycles",
			Buckets:   []float64{0.001, 0.01, 0.1, 1.0, 10.0},
		},
		[]string{"gc_type"},
	)

	mc.allocatedBytes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "allocated_bytes_total",
			Help:      "Total bytes allocated",
		},
	)

	mc.systemCpuUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "cpu_usage_percent",
			Help:      "CPU usage percentage",
		},
	)

	// Component metrics
	mc.componentOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "component_operations_total",
			Help:      "Total number of component operations",
		},
		componentLabels,
	)

	mc.componentDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "component_operation_duration_seconds",
			Help:      "Duration of component operations in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		componentLabels,
	)

	mc.componentErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "component_errors_total",
			Help:      "Total number of component errors",
		},
		append(componentLabels, "error_type"),
	)

	// Storage metrics
	mc.storageQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "storage_queries_total",
			Help:      "Total number of storage queries",
		},
		storageLabels,
	)

	mc.storageConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "storage_connections_active",
			Help:      "Number of active storage connections",
		},
	)

	mc.storageLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "storage_query_duration_seconds",
			Help:      "Duration of storage queries in seconds",
			Buckets:   []float64{0.001, 0.01, 0.1, 1.0, 5.0},
		},
		storageLabels,
	)

	// Cache metrics
	mc.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
		cacheLabels,
	)

	mc.cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
		cacheLabels,
	)

	mc.cacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "cache_size_bytes",
			Help:      "Size of cache in bytes",
		},
		[]string{"cache_type"},
	)

	// Logging metrics
	mc.logMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "log_messages_total",
			Help:      "Total number of log messages",
		},
		logLabels,
	)

	mc.logErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "log_errors_total",
			Help:      "Total number of log errors",
		},
		logLabels,
	)

	mc.logDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "log_messages_dropped_total",
			Help:      "Total number of dropped log messages",
		},
		logLabels,
	)

	mc.auditEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mc.config.Namespace,
			Subsystem: mc.config.Subsystem,
			Name:      "audit_events_total",
			Help:      "Total number of audit events",
		},
		[]string{"event_type", "result"},
	)
}

// Request Metrics

// RecordRequest records HTTP request metrics
func (mc *MetricsCollector) RecordRequest(method, endpoint, statusCode string, duration time.Duration, requestSize, responseSize int64) {
	if mc == nil {
		return
	}

	mc.requestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration.Seconds())
	mc.requestCounter.WithLabelValues(method, endpoint, statusCode).Inc()
	mc.requestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
	mc.responseSize.WithLabelValues(method, endpoint, statusCode).Observe(float64(responseSize))
}

// IncActiveRequests increments active request count
func (mc *MetricsCollector) IncActiveRequests(method, endpoint string) {
	if mc == nil {
		return
	}
	mc.activeRequests.WithLabelValues(method, endpoint).Inc()
}

// DecActiveRequests decrements active request count
func (mc *MetricsCollector) DecActiveRequests(method, endpoint string) {
	if mc == nil {
		return
	}
	mc.activeRequests.WithLabelValues(method, endpoint).Dec()
}

// Component Metrics

// RecordComponentOperation records component operation metrics
func (mc *MetricsCollector) RecordComponentOperation(component, operation string, duration time.Duration) {
	if mc == nil {
		return
	}

	mc.componentOperations.WithLabelValues(component, operation).Inc()
	mc.componentDuration.WithLabelValues(component, operation).Observe(duration.Seconds())
}

// RecordComponentError records component error metrics
func (mc *MetricsCollector) RecordComponentError(component, operation, errorType string) {
	if mc == nil {
		return
	}
	mc.componentErrors.WithLabelValues(component, operation, errorType).Inc()
}

// Storage Metrics

// RecordStorageQuery records storage query metrics
func (mc *MetricsCollector) RecordStorageQuery(operation, table string, duration time.Duration) {
	if mc == nil {
		return
	}

	mc.storageQueries.WithLabelValues(operation, table).Inc()
	mc.storageLatency.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// SetStorageConnections sets the number of active storage connections
func (mc *MetricsCollector) SetStorageConnections(count int) {
	if mc == nil {
		return
	}
	mc.storageConnections.Set(float64(count))
}

// Cache Metrics

// RecordCacheHit records cache hit metrics
func (mc *MetricsCollector) RecordCacheHit(cacheType, keyPattern string) {
	if mc == nil {
		return
	}
	mc.cacheHits.WithLabelValues(cacheType, keyPattern).Inc()
}

// RecordCacheMiss records cache miss metrics
func (mc *MetricsCollector) RecordCacheMiss(cacheType, keyPattern string) {
	if mc == nil {
		return
	}
	mc.cacheMisses.WithLabelValues(cacheType, keyPattern).Inc()
}

// SetCacheSize sets cache size metrics
func (mc *MetricsCollector) SetCacheSize(cacheType string, sizeBytes int64) {
	if mc == nil {
		return
	}
	mc.cacheSize.WithLabelValues(cacheType).Set(float64(sizeBytes))
}

// Logging Metrics

// RecordLogMessage records log message metrics
func (mc *MetricsCollector) RecordLogMessage(level, component string) {
	if mc == nil {
		return
	}
	mc.logMessages.WithLabelValues(level, component).Inc()
}

// RecordLogError records log error metrics
func (mc *MetricsCollector) RecordLogError(level, component string) {
	if mc == nil {
		return
	}
	mc.logErrors.WithLabelValues(level, component).Inc()
}

// RecordLogDropped records dropped log message metrics
func (mc *MetricsCollector) RecordLogDropped(level, component string) {
	if mc == nil {
		return
	}
	mc.logDropped.WithLabelValues(level, component).Inc()
}

// RecordAuditEvent records audit event metrics
func (mc *MetricsCollector) RecordAuditEvent(eventType, result string) {
	if mc == nil {
		return
	}
	mc.auditEvents.WithLabelValues(eventType, result).Inc()
}

// collectRuntimeMetrics collects Go runtime metrics periodically
func (mc *MetricsCollector) collectRuntimeMetrics() {
	ticker := time.NewTicker(mc.config.CollectInterval)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		mc.memoryUsage.Set(float64(m.Alloc))
		mc.goroutineCount.Set(float64(runtime.NumGoroutine()))
		mc.allocatedBytes.Set(float64(m.TotalAlloc))

		// Record GC metrics if available
		if len(m.PauseNs) > 0 {
			// Get the most recent GC pause
			lastPause := time.Duration(m.PauseNs[(m.NumGC+255)%256])
			mc.gcDuration.WithLabelValues("mark_sweep").Observe(lastPause.Seconds())
		}
	}
}

// GetHTTPHandler returns the HTTP handler for metrics endpoint
func (mc *MetricsCollector) GetHTTPHandler() http.Handler {
	if mc == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(mc.gatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		ErrorHandling:     promhttp.ContinueOnError,
	})
}

// StartMetricsServer starts the metrics HTTP server
func (mc *MetricsCollector) StartMetricsServer() error {
	if mc == nil || mc.config.ListenAddr == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle(mc.config.Path, mc.GetHTTPHandler())
	
	server := &http.Server{
		Addr:    mc.config.ListenAddr,
		Handler: mux,
	}

	return server.ListenAndServe()
}

// RequestTimer provides a convenient way to time operations
type RequestTimer struct {
	collector *MetricsCollector
	component string
	operation string
	startTime time.Time
}

// NewRequestTimer creates a new request timer
func (mc *MetricsCollector) NewRequestTimer(component, operation string) *RequestTimer {
	return &RequestTimer{
		collector: mc,
		component: component,
		operation: operation,
		startTime: time.Now(),
	}
}

// Finish records the operation duration and optionally an error
func (rt *RequestTimer) Finish(err error) {
	if rt == nil || rt.collector == nil {
		return
	}

	duration := time.Since(rt.startTime)
	rt.collector.RecordComponentOperation(rt.component, rt.operation, duration)

	if err != nil {
		rt.collector.RecordComponentError(rt.component, rt.operation, "error")
	}
}

// TimeOperation times an operation using a closure
func (mc *MetricsCollector) TimeOperation(component, operation string, fn func() error) error {
	timer := mc.NewRequestTimer(component, operation)
	defer func() {
		timer.Finish(nil)
	}()

	err := fn()
	if err != nil {
		mc.RecordComponentError(component, operation, "error")
	}

	return err
}

// Middleware returns HTTP middleware for automatic request metrics
func (mc *MetricsCollector) Middleware() func(http.Handler) http.Handler {
	if mc == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Track active requests
			mc.IncActiveRequests(r.Method, r.URL.Path)
			defer mc.DecActiveRequests(r.Method, r.URL.Path)

			// Wrap response writer to capture status and size
			wrapped := &responseWrapper{
				ResponseWriter: w,
				statusCode:     200,
			}

			// Serve request
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start)
			requestSize := r.ContentLength
			if requestSize < 0 {
				requestSize = 0
			}

			mc.RecordRequest(
				r.Method,
				r.URL.Path,
				http.StatusText(wrapped.statusCode),
				duration,
				requestSize,
				int64(wrapped.responseSize),
			)
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture metrics
type responseWrapper struct {
	http.ResponseWriter
	statusCode   int
	responseSize int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.responseSize += size
	return size, err
}

// Close closes the metrics collector and stops metric collection
func (mc *MetricsCollector) Close() error {
	if mc == nil {
		return nil
	}
	// Cleanup resources if needed
	return nil
}