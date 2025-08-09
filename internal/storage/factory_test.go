package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBackend_SQLite(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &config.Settings{
		StorageType: "sqlite",
		StoragePath: dbPath,
		Sqlite: config.SqliteSettings{
			WALMode: true,
		},
	}

	backend, err := NewBackend(cfg)
	require.NoError(t, err)
	require.NotNil(t, backend)
	defer backend.Close()

	// Verify it's actually an SQLite backend by checking the type
	_, ok := backend.(*SqliteBackend)
	assert.True(t, ok, "Expected SqliteBackend type")

	// Verify the database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")
}

func TestNewBackend_SQLite_NoPath(t *testing.T) {
	cfg := &config.Settings{
		StorageType: "sqlite",
		StoragePath: "", // Empty path should cause error
	}

	backend, err := NewBackend(cfg)
	assert.Error(t, err)
	assert.Nil(t, backend)
	assert.Contains(t, err.Error(), "storage path is required")
}

func TestNewBackend_Memory(t *testing.T) {
	testCases := []struct {
		name        string
		storageType string
	}{
		{"explicit memory", "memory"},
		{"empty defaults to memory", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Settings{
				StorageType: tc.storageType,
			}

			backend, err := NewBackend(cfg)
			require.NoError(t, err)
			require.NotNil(t, backend)
			defer backend.Close()

			// Verify it's actually a memory backend
			_, ok := backend.(*MemoryBackend)
			assert.True(t, ok, "Expected MemoryBackend type")
		})
	}
}

func TestNewBackend_UnsupportedType(t *testing.T) {
	cfg := &config.Settings{
		StorageType: "postgresql", // Unsupported type
	}

	backend, err := NewBackend(cfg)
	assert.Error(t, err)
	assert.Nil(t, backend)
	assert.Contains(t, err.Error(), "unsupported storage type: postgresql")
}

func TestNewBackend_IntegrationWithOperations(t *testing.T) {
	// Test that the created backends actually work
	testCases := []struct {
		name   string
		config *config.Settings
	}{
		{
			name: "SQLite backend operations",
			config: &config.Settings{
				StorageType: "sqlite",
				StoragePath: filepath.Join(t.TempDir(), "test.db"),
				Sqlite:      config.SqliteSettings{WALMode: true},
			},
		},
		{
			name: "Memory backend operations",
			config: &config.Settings{
				StorageType: "memory",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := NewBackend(tc.config)
			require.NoError(t, err)
			require.NotNil(t, backend)
			defer backend.Close()

			// Test basic operations work
			ctx := context.Background()
			
			// Create an entity
			entities := []mcp.Entity{
				{
					ID:   "test-1",
					Name: "Test Entity",
				},
			}
			err = backend.CreateEntities(ctx, entities)
			assert.NoError(t, err)

			// Retrieve the entity
			retrieved, err := backend.GetEntity(ctx, "test-1")
			assert.NoError(t, err)
			assert.NotNil(t, retrieved)
			assert.Equal(t, "test-1", retrieved.ID)
			assert.Equal(t, "Test Entity", retrieved.Name)

			// Search for entities
			results, err := backend.SearchEntities(ctx, "Test")
			assert.NoError(t, err)
			assert.Len(t, results, 1)

			// Get statistics
			stats, err := backend.GetStatistics(ctx)
			assert.NoError(t, err)
			assert.Equal(t, 1, stats["entities"])
		})
	}
}