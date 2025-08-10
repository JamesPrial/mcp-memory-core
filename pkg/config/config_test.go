package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Success(t *testing.T) {
	content := `
storageType: "sqlite"
storagePath: "/var/data/test"
httpPort: 8080
logLevel: "debug"
sqlite:
  walMode: true
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	assert.NoError(t, err)

	cfg, err := Load(configPath)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "sqlite", cfg.StorageType)
	assert.Equal(t, "/var/data/test", cfg.StoragePath)
	assert.Equal(t, 8080, cfg.HTTPPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.True(t, cfg.Sqlite.WALMode)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("non_existent_file.yaml")
	assert.Error(t, err)
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `[invalid yaml - unclosed bracket`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	assert.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
}

func TestValidate_ValidConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
	}{
		{
			name: "valid sqlite configuration",
			settings: Settings{
				StorageType: "sqlite",
				StoragePath: "/var/data/test",
				HTTPPort:    8080,
				LogLevel:    "info",
				Sqlite:      SqliteSettings{WALMode: true},
			},
		},
		{
			name: "valid memory configuration",
			settings: Settings{
				StorageType: "memory",
				StoragePath: "", // empty path is OK for memory storage
				HTTPPort:    3000,
				LogLevel:    "debug",
				Sqlite:      SqliteSettings{WALMode: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestValidate_CaseInsensitiveLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected string // normalized value
	}{
		{"uppercase INFO", "INFO", "info"},
		{"uppercase DEBUG", "DEBUG", "debug"},
		{"mixed case Warn", "Warn", "warn"},
		{"mixed case ErRoR", "ErRoR", "error"},
		{"empty string allowed", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				StorageType: "memory",
				LogLevel:    tt.logLevel,
			}

			err := settings.Validate()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, settings.LogLevel)
		})
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"invalid level", "invalid"},
		{"fatal not allowed", "fatal"},
		{"trace not allowed", "trace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				StorageType: "memory",
				StoragePath: "/some/path",
				HTTPPort:    8080,
				LogLevel:    tt.logLevel,
				Sqlite:      SqliteSettings{WALMode: true},
			}

			err := settings.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "logLevel must be one of [debug, info, warn, error]")
			assert.Contains(t, err.Error(), fmt.Sprintf("got '%s'", tt.logLevel))
		})
	}
}

func TestValidate_CaseInsensitiveStorageType(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		expected    string // normalized value
	}{
		{"uppercase MEMORY", "MEMORY", "memory"},
		{"uppercase SQLITE", "SQLITE", "sqlite"},
		{"mixed case Memory", "Memory", "memory"},
		{"mixed case SqLiTe", "SqLiTe", "sqlite"},
		{"empty string allowed", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				StorageType: tt.storageType,
				StoragePath: "/some/path", // Required for sqlite
			}

			err := settings.Validate()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, settings.StorageType)
		})
	}
}

func TestValidate_InvalidStorageType(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
	}{
		{"postgresql not supported", "postgresql"},
		{"redis not supported", "redis"},
		{"mysql not supported", "mysql"},
		{"invalid type", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				StorageType: tt.storageType,
			}

			err := settings.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "storageType must be one of [memory, sqlite]")
		})
	}
}

func TestValidate_HTTPPort(t *testing.T) {
	tests := []struct {
		name        string
		port        int
		shouldError bool
	}{
		{"valid port 8080", 8080, false},
		{"valid port 80", 80, false},
		{"valid port 443", 443, false},
		{"valid port 3000", 3000, false},
		{"valid port 65535", 65535, false},
		{"valid port 0 (any)", 0, false},
		{"negative port", -1, true},
		{"port too high", 65536, true},
		{"port way too high", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				StorageType: "memory",
				HTTPPort:    tt.port,
			}

			err := settings.Validate()
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "httpPort must be between 0 and 65535")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_EmptySQLitePath(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		storagePath string
		shouldError bool
	}{
		{
			name:        "sqlite with empty path",
			storageType: "sqlite",
			storagePath: "",
			shouldError: true,
		},
		{
			name:        "sqlite with whitespace only path",
			storageType: "sqlite",
			storagePath: "   ",
			shouldError: true,
		},
		{
			name:        "sqlite with valid path",
			storageType: "sqlite",
			storagePath: "/var/data/test.db",
			shouldError: false,
		},
		{
			name:        "memory with empty path (should be OK)",
			storageType: "memory",
			storagePath: "",
			shouldError: false,
		},
		{
			name:        "SQLite uppercase with empty path",
			storageType: "SQLITE",
			storagePath: "",
			shouldError: true,
		},
		{
			name:        "mixed case sqlite with empty path",
			storageType: "SQLite",
			storagePath: "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := Settings{
				StorageType: tt.storageType,
				StoragePath: tt.storagePath,
				HTTPPort:    8080,
				LogLevel:    "info",
				Sqlite:      SqliteSettings{WALMode: true},
			}

			err := settings.Validate()
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "storagePath cannot be empty when storageType is sqlite")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_ValidLogLevels(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error"}

	for _, level := range validLevels {
		t.Run("valid_level_"+level, func(t *testing.T) {
			settings := Settings{
				StorageType: "memory",
				StoragePath: "/some/path",
				HTTPPort:    8080,
				LogLevel:    level,
				Sqlite:      SqliteSettings{WALMode: true},
			}

			err := settings.Validate()
			assert.NoError(t, err)
		})
	}
}

// Integration test using Load function to test validation during config loading
func TestLoad_ValidationFailure_InvalidLogLevel(t *testing.T) {
	content := `
storageType: "sqlite"
storagePath: "/var/data/test"
httpPort: 8080
logLevel: "invalid"
sqlite:
  walMode: true
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	assert.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
	assert.Contains(t, err.Error(), "logLevel must be one of [debug, info, warn, error], got 'invalid'")
}

// Integration test for SQLite path validation during config loading
func TestLoad_ValidationFailure_EmptySQLitePath(t *testing.T) {
	content := `
storageType: "sqlite"
storagePath: ""
httpPort: 8080
logLevel: "info"
sqlite:
  walMode: true
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	assert.NoError(t, err)

	_, err = Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
	assert.Contains(t, err.Error(), "storagePath cannot be empty when storageType is sqlite")
}

// Test boundary conditions and edge cases
func TestValidate_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		settings    Settings
		shouldError bool
		errorMsg    string
	}{
		{
			name: "empty log level is allowed",
			settings: Settings{
				StorageType: "memory",
				LogLevel:    "",
				HTTPPort:    8080,
			},
			shouldError: false,
			errorMsg:    "",
		},
		{
			name: "log level with extra spaces",
			settings: Settings{
				StorageType: "memory",
				LogLevel:    " info ",
				HTTPPort:    8080,
			},
			shouldError: true,
			errorMsg:    "logLevel must be one of [debug, info, warn, error], got ' info '",
		},
		{
			name: "sqlite with path containing only tabs",
			settings: Settings{
				StorageType: "sqlite",
				StoragePath: "\t\t\t",
				LogLevel:    "info",
				HTTPPort:    8080,
			},
			shouldError: true,
			errorMsg:    "storagePath cannot be empty when storageType is sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}