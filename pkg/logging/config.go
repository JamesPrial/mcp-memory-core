package logging

import (
	"fmt"
	"strings"
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
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Endpoint    string `yaml:"endpoint" json:"endpoint"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Insecure    bool   `yaml:"insecure" json:"insecure"`
	Timeout     int    `yaml:"timeout" json:"timeout"` // seconds
	BatchSize   int    `yaml:"batchSize" json:"batchSize"`
	QueueSize   int    `yaml:"queueSize" json:"queueSize"`
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
			Enabled:   false,
			Timeout:   30,
			BatchSize: 100,
			QueueSize: 1000,
		},
		
		Metrics: DefaultMetricsConfig(),
		
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
	if c.OTLP.Enabled && strings.TrimSpace(c.OTLP.Endpoint) == "" {
		return fmt.Errorf("OTLP endpoint required when OTLP is enabled")
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