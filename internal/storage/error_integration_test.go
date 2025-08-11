package storage

import (
	"context"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorHandlingIntegration tests that the standardized error handling works across the storage layer
func TestErrorHandlingIntegration(t *testing.T) {
	t.Run("Factory Configuration Errors", func(t *testing.T) {
		// Test nil config
		backend, err := NewBackend(nil)
		assert.Nil(t, backend)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeConfiguration), "Expected configuration error")
		
		// Test unsupported storage type
		cfg := &config.Settings{StorageType: "unsupported"}
		backend, err = NewBackend(cfg)
		assert.Nil(t, backend)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeConfiguration), "Expected configuration error")
		
		// Test SQLite without path
		cfg = &config.Settings{StorageType: "sqlite", StoragePath: ""}
		backend, err = NewBackend(cfg)
		assert.Nil(t, backend)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeConfiguration), "Expected configuration error")
	})
	
	t.Run("Memory Backend Validation Errors", func(t *testing.T) {
		backend := NewMemoryBackend()
		ctx := context.Background()
		
		// Test empty entity ID
		entities := []mcp.Entity{{ID: "", Name: "Test"}}
		err := backend.CreateEntities(ctx, entities)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeValidationRequired), "Expected validation required error")
		
		// Test duplicate entity ID
		entities = []mcp.Entity{{ID: "test-1", Name: "Test 1"}}
		err = backend.CreateEntities(ctx, entities)
		assert.NoError(t, err)
		
		// Try to create again with same ID
		err = backend.CreateEntities(ctx, entities)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeEntityAlreadyExists), "Expected entity already exists error")
	})
	
	t.Run("SQLite Backend Connection Errors", func(t *testing.T) {
		// Test invalid path that should fail
		_, err := NewSqliteBackend("/invalid/nonexistent/path/test.db", true)
		if err != nil {
			assert.True(t, errors.Is(err, errors.ErrCodeStorageConnection), "Expected storage connection error")
		}
	})
	
	t.Run("Error Message Safety", func(t *testing.T) {
		// Test that error messages are safe for client consumption
		backend := NewMemoryBackend()
		ctx := context.Background()
		
		entities := []mcp.Entity{{ID: "", Name: "Test"}}
		err := backend.CreateEntities(ctx, entities)
		
		assert.Error(t, err)
		msg := errors.GetMessage(err)
		assert.NotEmpty(t, msg)
		assert.NotContains(t, msg, "internal", "Internal error details should not leak")
		
		code := errors.GetCode(err)
		assert.Equal(t, errors.ErrCodeValidationRequired, code)
	})
}

// TestErrorCodeConsistency ensures error codes are consistent across different backends
func TestErrorCodeConsistency(t *testing.T) {
	memBackend := NewMemoryBackend()
	
	// Create SQLite backend for comparison
	tempDir := t.TempDir()
	sqliteBackend, err := NewSqliteBackend(tempDir+"/test.db", true)
	require.NoError(t, err)
	defer sqliteBackend.Close()
	
	ctx := context.Background()
	
	t.Run("Validation Errors Are Consistent", func(t *testing.T) {
		entities := []mcp.Entity{{ID: "", Name: "Test"}}
		
		// Memory backend validation error
		memErr := memBackend.CreateEntities(ctx, entities)
		assert.Error(t, memErr)
		assert.True(t, errors.Is(memErr, errors.ErrCodeValidationRequired), "Memory backend should return validation error")
		
		// SQLite backend validation error
		sqliteErr := sqliteBackend.CreateEntities(ctx, entities)
		assert.Error(t, sqliteErr)
		assert.True(t, errors.Is(sqliteErr, errors.ErrCodeValidationRequired), "SQLite backend should return validation error")
		
		// Both should have the same error code
		assert.Equal(t, errors.GetCode(memErr), errors.GetCode(sqliteErr), "Error codes should be consistent across backends")
	})
	
	t.Run("Not Found Behavior Is Consistent", func(t *testing.T) {
		// Both backends should return nil for non-existent entities
		memEntity, memErr := memBackend.GetEntity(ctx, "nonexistent")
		assert.NoError(t, memErr)
		assert.Nil(t, memEntity)
		
		sqliteEntity, sqliteErr := sqliteBackend.GetEntity(ctx, "nonexistent")
		assert.NoError(t, sqliteErr)
		assert.Nil(t, sqliteEntity)
	})
}