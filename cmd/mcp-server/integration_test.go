package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/internal/transport"
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerWithSQLiteBackend(t *testing.T) {
	// Create temp directory for SQLite database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create config file
	configContent := `storageType: "sqlite"
storagePath: "` + dbPath + `"
httpPort: 8080
logLevel: "info"
sqlite:
  walMode: true`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load configuration
	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "sqlite", cfg.StorageType)
	assert.Equal(t, dbPath, cfg.StoragePath)

	// Create storage backend using factory
	backend, err := storage.NewBackend(cfg)
	require.NoError(t, err)
	require.NotNil(t, backend)
	defer backend.Close()

	// Verify it's actually an SQLite backend
	_, ok := backend.(*storage.SqliteBackend)
	assert.True(t, ok, "Expected SqliteBackend type")

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")

	// Create knowledge manager and server
	manager := knowledge.NewManager(backend)
	server := NewServer(manager)

	// Test tools/list
	ctx := context.Background()
	req := &transport.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	resp := server.HandleRequest(ctx, req)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)

	// Test creating entities
	req = &transport.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "memory__create_entities",
			"arguments": map[string]interface{}{
				"entities": []interface{}{
					map[string]interface{}{
						"name": "Test Entity",
						"entityType": "test",
					},
				},
			},
		},
	}
	resp = server.HandleRequest(ctx, req)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)

	// Test statistics to verify entity was created
	req = &transport.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "memory__get_statistics",
			"arguments": map[string]interface{}{},
		},
	}
	resp = server.HandleRequest(ctx, req)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)

	// Verify the statistics show 1 entity
	if statsMap, ok := resp.Result.(map[string]int); ok {
		assert.Equal(t, 1, statsMap["entities"], "Should have 1 entity")
	}
}

func TestServerWithMemoryBackend(t *testing.T) {
	// Create temp directory for config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create config file for memory backend
	configContent := `storageType: "memory"
httpPort: 8080
logLevel: "info"`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load configuration
	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "memory", cfg.StorageType)

	// Create storage backend using factory
	backend, err := storage.NewBackend(cfg)
	require.NoError(t, err)
	require.NotNil(t, backend)
	defer backend.Close()

	// Verify it's actually a memory backend
	_, ok := backend.(*storage.MemoryBackend)
	assert.True(t, ok, "Expected MemoryBackend type")

	// Create knowledge manager and server
	manager := knowledge.NewManager(backend)
	server := NewServer(manager)

	// Test basic operations
	ctx := context.Background()
	req := &transport.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	resp := server.HandleRequest(ctx, req)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)
}