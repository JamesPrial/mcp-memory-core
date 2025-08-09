// In file: pkg/config/config_test.go
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