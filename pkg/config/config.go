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
	// Validate LogLevel - must be one of [debug, info, warn, error]
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[s.LogLevel] {
		return fmt.Errorf("logLevel must be one of [debug, info, warn, error], got '%s'", s.LogLevel)
	}

	// Validate SQLite path - if storageType is sqlite, storagePath must not be empty
	if strings.ToLower(s.StorageType) == "sqlite" && strings.TrimSpace(s.StoragePath) == "" {
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