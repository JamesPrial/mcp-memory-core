package config

import (
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

func TestLoad_InvalidHTTPPort(t *testing.T) {
	tests := []struct {
		name     string
		httpPort int
		wantErr  string
	}{
		{
			name:     "negative port",
			httpPort: -1,
			wantErr:  "httpPort must be between 1 and 65535, got -1",
		},
		{
			name:     "zero port",
			httpPort: 0,
			wantErr:  "httpPort must be between 1 and 65535, got 0",
		},
		{
			name:     "port too high",
			httpPort: 65536,
			wantErr:  "httpPort must be between 1 and 65535, got 65536",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `
storageType: "sqlite"
storagePath: "/var/data/test"
httpPort: ` + fmt.Sprintf("%d", tt.httpPort) + `
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
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
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
	assert.Contains(t, err.Error(), "logLevel must be one of [debug, info, warn, error, fatal], got 'invalid'")
}

func TestLoad_InvalidStorageType(t *testing.T) {
	content := `
storageType: "unsupported"
storagePath: "/var/data/test"
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
	assert.Contains(t, err.Error(), "storageType must be one of [sqlite, memory], got 'unsupported'")
}

func TestLoad_ValidLogLevels(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	
	for _, level := range validLevels {
		t.Run("valid_level_"+level, func(t *testing.T) {
			content := `
storageType: "sqlite"
storagePath: "/var/data/test"
httpPort: 8080
logLevel: "` + level + `"
sqlite:
  walMode: true
`
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(content), 0644)
			assert.NoError(t, err)

			cfg, err := Load(configPath)
			assert.NoError(t, err)
			assert.Equal(t, level, cfg.LogLevel)
		})
	}
}

func TestLoad_ValidStorageTypes(t *testing.T) {
	validTypes := []string{"sqlite", "memory"}
	
	for _, storageType := range validTypes {
		t.Run("valid_type_"+storageType, func(t *testing.T) {
			content := `
storageType: "` + storageType + `"
storagePath: "/var/data/test"
httpPort: 8080
logLevel: "info"
sqlite:
  walMode: true
`
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(content), 0644)
			assert.NoError(t, err)

			cfg, err := Load(configPath)
			assert.NoError(t, err)
			assert.Equal(t, storageType, cfg.StorageType)
		})
	}
}

func TestValidate_ValidConfiguration(t *testing.T) {
	settings := &Settings{
		StorageType: "sqlite",
		StoragePath: "/var/data/test",
		HTTPPort:    8080,
		LogLevel:    "info",
		Sqlite:      SqliteSettings{WALMode: true},
	}

	err := settings.Validate()
	assert.NoError(t, err)
}