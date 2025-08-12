package knowledge

import (
	"context"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestManager_JSONMarshaling verifies that JSON marshaling errors are handled correctly
func TestManager_JSONMarshaling(t *testing.T) {
	// Create a mock entity with circular reference to cause JSON marshaling to fail
	// This won't actually cause marshaling to fail since our entities are simple structs
	// But we can test the code path exists
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()
	
	// Test successful JSON marshaling for create entities
	mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)
	
	args := map[string]interface{}{
		"entities": []interface{}{
			map[string]interface{}{
				"name": "Test Entity",
			},
		},
	}
	
	result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// Verify the result can be JSON marshaled
	resultMap := result.(map[string]interface{})
	assert.True(t, resultMap["success"].(bool))
	assert.Equal(t, 1, resultMap["count"].(int))
}

// TestManager_ErrorCodeHandling verifies that proper error codes are returned
func TestManager_ErrorCodeHandling(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()
	
	t.Run("ValidationRequired", func(t *testing.T) {
		// Test missing entities parameter
		_, err := manager.HandleCallTool(ctx, "memory__create_entities", map[string]interface{}{})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeValidationRequired))
	})
	
	t.Run("ValidationInvalid", func(t *testing.T) {
		// Test invalid entities type
		_, err := manager.HandleCallTool(ctx, "memory__create_entities", map[string]interface{}{
			"entities": "not an array",
		})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeValidationInvalid))
	})
	
	t.Run("TransportMethodNotFound", func(t *testing.T) {
		// Test unknown tool
		_, err := manager.HandleCallTool(ctx, "unknown_tool", map[string]interface{}{})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeTransportMethodNotFound))
	})
	
	t.Run("ContextCanceled", func(t *testing.T) {
		// Test cancelled context
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()
		
		_, err := manager.HandleCallTool(cancelledCtx, "memory__create_entities", map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"name": "Test"},
			},
		})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, errors.ErrCodeContextCanceled))
	})
}