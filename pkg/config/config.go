// In file: pkg/config/config.go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	StorageType string       `yaml:"storageType"`
	StoragePath string       `yaml:"storagePath"`
	HTTPPort    int          `yaml:"httpPort"`
	LogLevel    string       `yaml:"logLevel"`
	Sqlite      SqliteSettings `yaml:"sqlite"`
}

type SqliteSettings struct {
	WALMode bool `yaml:"walMode"`
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

	return &settings, nil
}