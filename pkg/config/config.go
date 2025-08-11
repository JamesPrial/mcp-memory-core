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