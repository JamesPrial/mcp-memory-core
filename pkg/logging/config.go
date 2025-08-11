package logging

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// LogFormat represents the output format for logs
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// LogOutput represents the destination for logs
type LogOutput string

const (
	LogOutputStdout LogOutput = "stdout"
	LogOutputStderr LogOutput = "stderr"
	LogOutputFile   LogOutput = "file"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Config represents the complete logging configuration
type Config struct {
	// Global settings
	Level  LogLevel  `yaml:"level" json:"level"`
	Format LogFormat `yaml:"format" json:"format"`
	Output LogOutput `yaml:"output" json:"output"`
	
	// Output settings
	FilePath string `yaml:"filePath,omitempty" json:"filePath,omitempty"`
	
	// Component-specific log levels
	ComponentLevels map[string]LogLevel `yaml:"componentLevels,omitempty" json:"componentLevels,omitempty"`
	
	// Performance settings
	BufferSize    int  `yaml:"bufferSize,omitempty" json:"bufferSize,omitempty"`
	AsyncLogging  bool `yaml:"asyncLogging,omitempty" json:"asyncLogging,omitempty"`
	
	// Sampling configuration
	Sampling SamplingConfig `yaml:"sampling,omitempty" json:"sampling,omitempty"`
	
	// Sensitive data handling
	Masking MaskingConfig `yaml:"masking,omitempty" json:"masking,omitempty"`
	
	// Request tracking
	EnableRequestID     bool `yaml:"enableRequestId" json:"enableRequestId"`
	EnableTracing       bool `yaml:"enableTracing" json:"enableTracing"`
	
	// Audit logging
	EnableAudit     bool   `yaml:"enableAudit" json:"enableAudit"`
	AuditFilePath   string `yaml:"auditFilePath,omitempty" json:"auditFilePath,omitempty"`
	
	// OpenTelemetry export
	OTLP OTLPConfig `yaml:"otlp,omitempty" json:"otlp,omitempty"`
	
	// Metrics collection
	Metrics MetricsConfig `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	
	// Distributed tracing
	// Tracing TracingConfig `yaml:"tracing,omitempty" json:"tracing,omitempty"`
	
	// Advanced sampling strategies
	AdvancedSampling AdvancedSamplingConfig `yaml:"advancedSampling,omitempty" json:"advancedSampling,omitempty"`
	
	// Health monitoring
	Health HealthConfig `yaml:"health,omitempty" json:"health,omitempty"`
	
	// Development settings
	EnableStackTrace bool `yaml:"enableStackTrace" json:"enableStackTrace"`
	EnableCaller     bool `yaml:"enableCaller" json:"enableCaller"`
	PrettyPrint      bool `yaml:"prettyPrint" json:"prettyPrint"`
}

// SamplingConfig defines log sampling rules
type SamplingConfig struct {
	Enabled      bool    `yaml:"enabled" json:"enabled"`
	Rate         float64 `yaml:"rate" json:"rate"`                 // 0.0 to 1.0
	BurstSize    int     `yaml:"burstSize" json:"burstSize"`       // Max burst before sampling kicks in
	AlwaysErrors bool    `yaml:"alwaysErrors" json:"alwaysErrors"` // Always log errors regardless of sampling
}

// MaskingConfig defines sensitive data masking rules
type MaskingConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Fields  []string `yaml:"fields" json:"fields"`   // Field names to mask
	Patterns []string `yaml:"patterns" json:"patterns"` // Regex patterns for value masking
	
	// Predefined patterns
	MaskEmails       bool `yaml:"maskEmails" json:"maskEmails"`
	MaskPhoneNumbers bool `yaml:"maskPhoneNumbers" json:"maskPhoneNumbers"`
	MaskCreditCards  bool `yaml:"maskCreditCards" json:"maskCreditCards"`
	MaskSSN          bool `yaml:"maskSsn" json:"maskSsn"`
	MaskAPIKeys      bool `yaml:"maskApiKeys" json:"maskApiKeys"`
}

// OTLPConfig defines OpenTelemetry export configuration
type OTLPConfig struct {
	Enabled                bool              `yaml:"enabled" json:"enabled"`
	Endpoint               string            `yaml:"endpoint" json:"endpoint"`
	Protocol               string            `yaml:"protocol,omitempty" json:"protocol,omitempty"` // "grpc" or "http", auto-detect if empty
	Headers                map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Insecure               bool              `yaml:"insecure" json:"insecure"`
	Timeout                int               `yaml:"timeout" json:"timeout"` // seconds
	BatchSize              int               `yaml:"batchSize" json:"batchSize"`
	BatchTimeout           int               `yaml:"batchTimeout" json:"batchTimeout"` // seconds
	QueueSize              int               `yaml:"queueSize" json:"queueSize"`
	HealthCheckInterval    int               `yaml:"healthCheckInterval" json:"healthCheckInterval"` // seconds
	TLS                    TLSConfig         `yaml:"tls,omitempty" json:"tls,omitempty"`
	RetryConfig            RetryConfig       `yaml:"retry,omitempty" json:"retry,omitempty"`
}

// TLSConfig defines TLS configuration for OTLP exports
type TLSConfig struct {
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
	ServerName         string `yaml:"serverName,omitempty" json:"serverName,omitempty"`
	CertFile           string `yaml:"certFile,omitempty" json:"certFile,omitempty"`
	KeyFile            string `yaml:"keyFile,omitempty" json:"keyFile,omitempty"`
	CAFile             string `yaml:"caFile,omitempty" json:"caFile,omitempty"`
}

// RetryConfig defines retry configuration for OTLP exports
type RetryConfig struct {
	Enabled         bool `yaml:"enabled" json:"enabled"`
	InitialInterval int  `yaml:"initialInterval" json:"initialInterval"` // milliseconds
	MaxInterval     int  `yaml:"maxInterval" json:"maxInterval"`         // milliseconds
	MaxElapsedTime  int  `yaml:"maxElapsedTime" json:"maxElapsedTime"`   // milliseconds
	MaxRetries      int  `yaml:"maxRetries" json:"maxRetries"`
}

// OTLPHealthStatus represents the health status of OTLP components
type OTLPHealthStatus struct {
	Healthy             bool      `json:"healthy"`
	ConnectionHealthy   bool      `json:"connection_healthy"`
	LastExportTime      time.Time `json:"last_export_time"`
	LastError           error     `json:"last_error,omitempty"`
	ExportAttempts      int64     `json:"export_attempts"`
	ExportSuccesses     int64     `json:"export_successes"`
	ExportFailures      int64     `json:"export_failures"`
}

// ExportStats represents export statistics
type ExportStats struct {
	TotalAttempts     int64     `json:"total_attempts"`
	TotalSuccesses    int64     `json:"total_successes"`
	TotalFailures     int64     `json:"total_failures"`
	SuccessRate       float64   `json:"success_rate"`
	LastExportTime    time.Time `json:"last_export_time"`
	LastError         error     `json:"last_error,omitempty"`
	QueueSize         int       `json:"queue_size"`
	QueueCapacity     int       `json:"queue_capacity"`
	BufferUtilization float64   `json:"buffer_utilization"`
}

// DefaultConfig returns a default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:  LogLevelInfo,
		Format: LogFormatJSON,
		Output: LogOutputStdout,
		
		BufferSize:   1024,
		AsyncLogging: false,
		
		EnableRequestID: true,
		EnableTracing:   false,
		
		EnableAudit: false,
		
		Sampling: SamplingConfig{
			Enabled:      false,
			Rate:         1.0,
			BurstSize:    100,
			AlwaysErrors: true,
		},
		
		Masking: MaskingConfig{
			Enabled:          true,
			MaskEmails:       true,
			MaskPhoneNumbers: true,
			MaskCreditCards:  true,
			MaskSSN:          true,
			MaskAPIKeys:      true,
		},
		
		OTLP: OTLPConfig{
			Enabled:             false,
			Timeout:             30,
			BatchSize:           100,
			BatchTimeout:        5,
			QueueSize:           1000,
			HealthCheckInterval: 30,
			TLS: TLSConfig{
				InsecureSkipVerify: false,
			},
			RetryConfig: RetryConfig{
				Enabled:         true,
				InitialInterval: 1000,  // 1 second
				MaxInterval:     30000, // 30 seconds
				MaxElapsedTime:  300000, // 5 minutes
				MaxRetries:      5,
			},
		},
		
		Metrics: DefaultMetricsConfig(),
		
		// Tracing: DefaultTracingConfig(),
		
		AdvancedSampling: DefaultAdvancedSamplingConfig(),
		
		Health: DefaultHealthConfig(),
		
		EnableStackTrace: false,
		EnableCaller:     false,
		PrettyPrint:      false,
	}
}

// DevelopmentConfig returns a configuration suitable for development
func DevelopmentConfig() *Config {
	config := DefaultConfig()
	config.Level = LogLevelDebug
	config.Format = LogFormatText
	config.EnableStackTrace = true
	config.EnableCaller = true
	config.PrettyPrint = true
	config.AsyncLogging = false
	config.Masking.Enabled = false // Disable masking in dev for easier debugging
	return config
}

// ProductionConfig returns a configuration suitable for production
func ProductionConfig() *Config {
	config := DefaultConfig()
	config.Level = LogLevelInfo
	config.Format = LogFormatJSON
	config.AsyncLogging = true
	config.BufferSize = 4096
	config.Sampling.Enabled = true
	config.Sampling.Rate = 0.1 // Sample 10% of non-error logs
	config.EnableAudit = true
	
	// Enable observability features in production
	config.OTLP.Enabled = true
	// config.Tracing.Enabled = true
	config.AdvancedSampling.Enabled = true
	config.Health.Enabled = true
	return config
}

// Validate validates the logging configuration
func (c *Config) Validate() error {
	// Validate log level
	validLevels := map[LogLevel]bool{
		LogLevelDebug: true,
		LogLevelInfo:  true,
		LogLevelWarn:  true,
		LogLevelError: true,
	}
	if !validLevels[c.Level] {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}
	
	// Validate component levels
	for component, level := range c.ComponentLevels {
		if !validLevels[level] {
			return fmt.Errorf("invalid log level for component %s: %s", component, level)
		}
	}
	
	// Validate format
	validFormats := map[LogFormat]bool{
		LogFormatJSON: true,
		LogFormatText: true,
	}
	if !validFormats[c.Format] {
		return fmt.Errorf("invalid log format: %s", c.Format)
	}
	
	// Validate output
	validOutputs := map[LogOutput]bool{
		LogOutputStdout: true,
		LogOutputStderr: true,
		LogOutputFile:   true,
	}
	if !validOutputs[c.Output] {
		return fmt.Errorf("invalid log output: %s", c.Output)
	}
	
	// Validate file paths
	if c.Output == LogOutputFile && strings.TrimSpace(c.FilePath) == "" {
		return fmt.Errorf("filePath required when output is 'file'")
	}
	
	if c.EnableAudit && strings.TrimSpace(c.AuditFilePath) == "" {
		c.AuditFilePath = "audit.log" // Default audit log path
	}
	
	// Validate sampling
	if c.Sampling.Enabled {
		if c.Sampling.Rate < 0.0 || c.Sampling.Rate > 1.0 {
			return fmt.Errorf("sampling rate must be between 0.0 and 1.0, got %f", c.Sampling.Rate)
		}
		if c.Sampling.BurstSize < 0 {
			return fmt.Errorf("sampling burst size must be non-negative, got %d", c.Sampling.BurstSize)
		}
	}
	
	// Validate OTLP
	if c.OTLP.Enabled {
		if strings.TrimSpace(c.OTLP.Endpoint) == "" {
			return fmt.Errorf("OTLP endpoint required when OTLP is enabled")
		}
		if err := c.OTLP.Validate(); err != nil {
			return fmt.Errorf("invalid OTLP config: %w", err)
		}
	}
	
	// Validate buffer size
	if c.BufferSize < 0 {
		return fmt.Errorf("buffer size must be non-negative, got %d", c.BufferSize)
	}
	
	return nil
}

// GetLevelForComponent returns the log level for a specific component
func (c *Config) GetLevelForComponent(component string) LogLevel {
	if level, ok := c.ComponentLevels[component]; ok {
		return level
	}
	return c.Level
}

// Validate validates the OTLP configuration
func (c *OTLPConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	
	// Validate endpoint
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("endpoint is required when OTLP is enabled")
	}
	
	// Validate endpoint format
	if _, err := url.Parse(c.Endpoint); err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}
	
	// Validate protocol
	if c.Protocol != "" && c.Protocol != "grpc" && c.Protocol != "http" {
		return fmt.Errorf("protocol must be 'grpc' or 'http', got '%s'", c.Protocol)
	}
	
	// Validate timeout
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %d", c.Timeout)
	}
	
	// Validate batch size
	if c.BatchSize <= 0 {
		return fmt.Errorf("batchSize must be positive, got %d", c.BatchSize)
	}
	
	// Validate batch timeout
	if c.BatchTimeout <= 0 {
		return fmt.Errorf("batchTimeout must be positive, got %d", c.BatchTimeout)
	}
	
	// Validate queue size
	if c.QueueSize <= 0 {
		return fmt.Errorf("queueSize must be positive, got %d", c.QueueSize)
	}
	
	// Validate health check interval
	if c.HealthCheckInterval <= 0 {
		return fmt.Errorf("healthCheckInterval must be positive, got %d", c.HealthCheckInterval)
	}
	
	// Validate TLS config
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("invalid TLS config: %w", err)
	}
	
	// Validate retry config
	if err := c.RetryConfig.Validate(); err != nil {
		return fmt.Errorf("invalid retry config: %w", err)
	}
	
	return nil
}

// Validate validates the TLS configuration
func (c *TLSConfig) Validate() error {
	// If cert file is provided, key file must also be provided
	if c.CertFile != "" && c.KeyFile == "" {
		return fmt.Errorf("keyFile is required when certFile is provided")
	}
	
	// If key file is provided, cert file must also be provided
	if c.KeyFile != "" && c.CertFile == "" {
		return fmt.Errorf("certFile is required when keyFile is provided")
	}
	
	return nil
}

// Validate validates the retry configuration
func (c *RetryConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	
	if c.InitialInterval <= 0 {
		return fmt.Errorf("initialInterval must be positive, got %d", c.InitialInterval)
	}
	
	if c.MaxInterval <= 0 {
		return fmt.Errorf("maxInterval must be positive, got %d", c.MaxInterval)
	}
	
	if c.InitialInterval > c.MaxInterval {
		return fmt.Errorf("initialInterval (%d) cannot be greater than maxInterval (%d)", c.InitialInterval, c.MaxInterval)
	}
	
	if c.MaxElapsedTime <= 0 {
		return fmt.Errorf("maxElapsedTime must be positive, got %d", c.MaxElapsedTime)
	}
	
	if c.MaxRetries < 0 {
		return fmt.Errorf("maxRetries must be non-negative, got %d", c.MaxRetries)
	}
	
	return nil
}