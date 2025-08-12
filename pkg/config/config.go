package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	StorageType   string           `yaml:"storageType"`
	StoragePath   string           `yaml:"storagePath"`
	HTTPPort      int              `yaml:"httpPort"`
	LogLevel      string           `yaml:"logLevel"`
	Sqlite        SqliteSettings   `yaml:"sqlite"`
	TransportType string           `yaml:"transportType"`
	Transport     TransportSettings `yaml:"transport"`
	Logging       *LoggingSettings  `yaml:"logging"`
}

type SqliteSettings struct {
	WALMode bool `yaml:"walMode"`
}

type TransportSettings struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	ReadTimeout       int    `yaml:"readTimeout"`
	WriteTimeout      int    `yaml:"writeTimeout"`
	MaxConnections    int    `yaml:"maxConnections"`
	EnableCORS        bool   `yaml:"enableCors"`
	SSEHeartbeatSecs  int    `yaml:"sseHeartbeatSecs"`
}

type LoggingSettings struct {
	Format           string                     `yaml:"format"`
	Output           string                     `yaml:"output"`
	FilePath         string                     `yaml:"filePath"`
	BufferSize       int                        `yaml:"bufferSize"`
	AsyncLogging     bool                       `yaml:"asyncLogging"`
	EnableRequestID  bool                       `yaml:"enableRequestId"`
	EnableTracing    bool                       `yaml:"enableTracing"`
	EnableAudit      bool                       `yaml:"enableAudit"`
	AuditFilePath    string                     `yaml:"auditFilePath"`
	EnableStackTrace bool                       `yaml:"enableStackTrace"`
	EnableCaller     bool                       `yaml:"enableCaller"`
	PrettyPrint      bool                       `yaml:"prettyPrint"`
	ComponentLevels  map[string]string          `yaml:"componentLevels"`
	Sampling         *LoggingSamplingSettings   `yaml:"sampling"`
	Masking          *LoggingMaskingSettings    `yaml:"masking"`
	OTLP             *LoggingOTLPSettings       `yaml:"otlp"`
}

type LoggingSamplingSettings struct {
	Enabled      bool    `yaml:"enabled"`
	Rate         float64 `yaml:"rate"`
	BurstSize    int     `yaml:"burstSize"`
	AlwaysErrors bool    `yaml:"alwaysErrors"`
}

type LoggingMaskingSettings struct {
	Enabled          bool     `yaml:"enabled"`
	Fields           []string `yaml:"fields"`
	Patterns         []string `yaml:"patterns"`
	MaskEmails       bool     `yaml:"maskEmails"`
	MaskPhoneNumbers bool     `yaml:"maskPhoneNumbers"`
	MaskCreditCards  bool     `yaml:"maskCreditCards"`
	MaskSSN          bool     `yaml:"maskSsn"`
	MaskAPIKeys      bool     `yaml:"maskApiKeys"`
}

type LoggingOTLPSettings struct {
	Enabled   bool              `yaml:"enabled"`
	Endpoint  string            `yaml:"endpoint"`
	Headers   map[string]string `yaml:"headers"`
	Insecure  bool              `yaml:"insecure"`
	Timeout   int               `yaml:"timeout"`
	BatchSize int               `yaml:"batchSize"`
	QueueSize int               `yaml:"queueSize"`
}

// Validate validates the configuration settings
func (s *Settings) Validate() error {
	// Validate LogLevel - must be one of [debug, info, warn, error] (case-insensitive)
	// Empty log level is allowed and will use default
	if s.LogLevel != "" {
		validLogLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		normalizedLogLevel := strings.ToLower(s.LogLevel)
		if !validLogLevels[normalizedLogLevel] {
			return fmt.Errorf("logLevel must be one of [debug, info, warn, error], got '%s'", s.LogLevel)
		}
		// Normalize the log level in the config
		s.LogLevel = normalizedLogLevel
	}

	// Validate StorageType - must be one of [memory, sqlite] (case-insensitive)
	validStorageTypes := map[string]bool{
		"memory": true,
		"sqlite": true,
		"":       true, // Empty defaults to memory
	}
	normalizedStorageType := strings.ToLower(s.StorageType)
	if !validStorageTypes[normalizedStorageType] {
		return fmt.Errorf("storageType must be one of [memory, sqlite], got '%s'", s.StorageType)
	}
	// Normalize the storage type
	s.StorageType = normalizedStorageType

	// Validate TransportType - must be one of [stdio, http, sse] (case-insensitive)
	validTransportTypes := map[string]bool{
		"stdio": true,
		"http":  true,
		"sse":   true,
		"":      true, // Empty defaults to stdio
	}
	normalizedTransportType := strings.ToLower(s.TransportType)
	if !validTransportTypes[normalizedTransportType] {
		return fmt.Errorf("transportType must be one of [stdio, http, sse], got '%s'", s.TransportType)
	}
	// Normalize the transport type
	s.TransportType = normalizedTransportType
	
	// Set default transport type if empty
	if s.TransportType == "" {
		s.TransportType = "stdio"
	}

	// Validate HTTPPort - must be a valid port number
	if s.HTTPPort < 0 || s.HTTPPort > 65535 {
		return fmt.Errorf("httpPort must be between 0 and 65535, got %d", s.HTTPPort)
	}
	
	// Validate logging configuration if present
	if err := s.validateLoggingConfig(); err != nil {
		return fmt.Errorf("logging configuration validation failed: %w", err)
	}

	// Validate SQLite path - if storageType is sqlite, storagePath must not be empty
	if normalizedStorageType == "sqlite" && strings.TrimSpace(s.StoragePath) == "" {
		return fmt.Errorf("storagePath cannot be empty when storageType is sqlite")
	}

	// Validate transport settings for HTTP/SSE
	if s.TransportType == "http" || s.TransportType == "sse" {
		// Set default port if not specified
		if s.Transport.Port == 0 {
			s.Transport.Port = 8080
		}
		// Validate port range
		if s.Transport.Port < 1 || s.Transport.Port > 65535 {
			return fmt.Errorf("transport.port must be between 1 and 65535, got %d", s.Transport.Port)
		}
		// Set default host if not specified
		if s.Transport.Host == "" {
			s.Transport.Host = "localhost"
		}
		// Set default timeouts if not specified
		if s.Transport.ReadTimeout == 0 {
			s.Transport.ReadTimeout = 30
		}
		if s.Transport.WriteTimeout == 0 {
			s.Transport.WriteTimeout = 30
		}
		// Set default SSE heartbeat if not specified
		if s.Transport.SSEHeartbeatSecs == 0 {
			s.Transport.SSEHeartbeatSecs = 30
		}
		// Set default max connections if not specified
		if s.Transport.MaxConnections == 0 {
			s.Transport.MaxConnections = 100
		}
	}

	return nil
}

// validateLoggingConfig validates the logging configuration
func (s *Settings) validateLoggingConfig() error {
	if s.Logging == nil {
		return nil // No logging config to validate
	}
	
	logging := s.Logging
	
	// Validate format if specified
	if logging.Format != "" {
		validFormats := map[string]bool{
			"json": true,
			"text": true,
		}
		if !validFormats[strings.ToLower(logging.Format)] {
			return fmt.Errorf("logging format must be 'json' or 'text', got '%s'", logging.Format)
		}
	}
	
	// Validate output if specified
	if logging.Output != "" {
		validOutputs := map[string]bool{
			"stdout": true,
			"stderr": true,
			"file":   true,
		}
		if !validOutputs[strings.ToLower(logging.Output)] {
			return fmt.Errorf("logging output must be 'stdout', 'stderr', or 'file', got '%s'", logging.Output)
		}
		
		// If output is file, filePath must be specified
		if strings.ToLower(logging.Output) == "file" && strings.TrimSpace(logging.FilePath) == "" {
			return fmt.Errorf("logging filePath must be specified when output is 'file'")
		}
	}
	
	// Validate buffer size
	if logging.BufferSize < 0 {
		return fmt.Errorf("logging bufferSize must be non-negative, got %d", logging.BufferSize)
	}
	
	// Validate component levels
	if logging.ComponentLevels != nil {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		for component, level := range logging.ComponentLevels {
			if !validLevels[strings.ToLower(level)] {
				return fmt.Errorf("invalid log level '%s' for component '%s', must be one of [debug, info, warn, error]", level, component)
			}
		}
	}
	
	// Validate sampling configuration
	if logging.Sampling != nil {
		if logging.Sampling.Rate < 0.0 || logging.Sampling.Rate > 1.0 {
			return fmt.Errorf("logging sampling rate must be between 0.0 and 1.0, got %f", logging.Sampling.Rate)
		}
		if logging.Sampling.BurstSize < 0 {
			return fmt.Errorf("logging sampling burstSize must be non-negative, got %d", logging.Sampling.BurstSize)
		}
	}
	
	// Validate masking configuration
	if logging.Masking != nil {
		// Fields and patterns should be valid if specified but we won't validate regex here
		// to avoid startup failures due to complex regex
	}
	
	// Validate OTLP configuration
	if logging.OTLP != nil {
		if logging.OTLP.Enabled && strings.TrimSpace(logging.OTLP.Endpoint) == "" {
			return fmt.Errorf("logging OTLP endpoint must be specified when OTLP is enabled")
		}
		if logging.OTLP.Timeout < 0 {
			return fmt.Errorf("logging OTLP timeout must be non-negative, got %d", logging.OTLP.Timeout)
		}
		if logging.OTLP.BatchSize < 0 {
			return fmt.Errorf("logging OTLP batchSize must be non-negative, got %d", logging.OTLP.BatchSize)
		}
		if logging.OTLP.QueueSize < 0 {
			return fmt.Errorf("logging OTLP queueSize must be non-negative, got %d", logging.OTLP.QueueSize)
		}
	}
	
	// Validate audit configuration
	if logging.EnableAudit && strings.TrimSpace(logging.AuditFilePath) == "" {
		// Set default audit file path
		logging.AuditFilePath = "audit.log"
	}
	
	// Set reasonable defaults for missing values
	if logging.BufferSize == 0 {
		logging.BufferSize = 1024
	}
	
	// Normalize format and output values
	if logging.Format != "" {
		logging.Format = strings.ToLower(logging.Format)
	}
	if logging.Output != "" {
		logging.Output = strings.ToLower(logging.Output)
	}
	
	// Normalize component level values
	if logging.ComponentLevels != nil {
		for component, level := range logging.ComponentLevels {
			logging.ComponentLevels[component] = strings.ToLower(level)
		}
	}
	
	return nil
}

func Load(path string) (*Settings, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings Settings
	err = yaml.Unmarshal(bytes, &settings)
	if err != nil {
		return nil, err
	}

	// Validate the configuration after unmarshaling
	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &settings, nil
}