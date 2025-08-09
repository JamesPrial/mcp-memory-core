package config

import (
	"fmt"
	"os"

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
	// Validate HTTPPort
	if s.HTTPPort < 1 || s.HTTPPort > 65535 {
		return fmt.Errorf("httpPort must be between 1 and 65535, got %d", s.HTTPPort)
	}

	// Validate LogLevel
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if !validLogLevels[s.LogLevel] {
		return fmt.Errorf("logLevel must be one of [debug, info, warn, error, fatal], got '%s'", s.LogLevel)
	}

	// Validate StorageType
	validStorageTypes := map[string]bool{
		"sqlite": true,
		"memory": true,
	}
	if !validStorageTypes[s.StorageType] {
		return fmt.Errorf("storageType must be one of [sqlite, memory], got '%s'", s.StorageType)
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