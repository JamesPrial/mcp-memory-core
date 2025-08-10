package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	StorageType string         `yaml:"storageType"`
	StoragePath string         `yaml:"storagePath"`
	HTTPPort    int            `yaml:"httpPort"`
	LogLevel    string         `yaml:"logLevel"`
	Sqlite      SqliteSettings `yaml:"sqlite"`
}

type SqliteSettings struct {
	WALMode bool `yaml:"walMode"`
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

	// Validate HTTPPort - must be a valid port number
	if s.HTTPPort < 0 || s.HTTPPort > 65535 {
		return fmt.Errorf("httpPort must be between 0 and 65535, got %d", s.HTTPPort)
	}

	// Validate SQLite path - if storageType is sqlite, storagePath must not be empty
	if normalizedStorageType == "sqlite" && strings.TrimSpace(s.StoragePath) == "" {
		return fmt.Errorf("storagePath cannot be empty when storageType is sqlite")
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