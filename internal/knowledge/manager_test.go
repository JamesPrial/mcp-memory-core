// In file: internal/knowledge/manager_test.go
package knowledge

import (
	"context"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestManager_HandleListTools(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)

	tools := manager.HandleListTools()
	assert.NotEmpty(t, tools)
}

func TestManager_HandleCallTool_CreateEntities(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)

	mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

	_, err := manager.HandleCallTool(context.Background(), "memory__create_entities", map[string]interface{}{
		"entities": []interface{}{
			map[string]interface{}{"name": "Test"},
		},
	})

	assert.NoError(t, err)
	mockStorage.AssertExpectations(t)
}

func TestManager_HandleCallTool_Search(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)

	query := "find me"
	expectedResults := []mcp.Entity{{ID: "e1", Name: "found"}}

	mockStorage.On("SearchEntities", mock.Anything, query).Return(expectedResults, nil)

	result, err := manager.HandleCallTool(context.Background(), "memory__search", map[string]interface{}{
		"query": query,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockStorage.AssertExpectations(t)
}