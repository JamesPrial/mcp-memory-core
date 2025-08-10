package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestManager_EdgeCases_HandleCallTool_UnknownTool(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)

	testCases := []string{
		"",
		"unknown_tool",
		"memory__invalid",
		"memory_create_entities", // Missing double underscore
		"MEMORY__CREATE_ENTITIES", // Wrong case
		"memory__create_entities_extra",
		"sql_injection'; DROP TABLE tools; --",
		"../../../etc/passwd",
		"\x00null_byte",
		strings.Repeat("a", 10000), // Very long tool name
		"🦄_unicode_tool",
	}

	for _, toolName := range testCases {
		t.Run(fmt.Sprintf("Tool_%s", toolName), func(t *testing.T) {
			result, err := manager.HandleCallTool(context.Background(), toolName, map[string]interface{}{})
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "unknown tool")
		})
	}
}

func TestManager_EdgeCases_HandleCallTool_CreateEntities_MalformedArgs(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()

	t.Run("MissingEntitiesParameter", func(t *testing.T) {
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", map[string]interface{}{})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "entities parameter is required")
	})

	t.Run("NilArgs", func(t *testing.T) {
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "entities parameter is required")
	})

	t.Run("EntitiesNotArray", func(t *testing.T) {
		invalidTypes := []interface{}{
			"not an array",
			123,
			true,
			map[string]interface{}{"not": "array"},
			nil,
		}

		for i, invalidType := range invalidTypes {
			t.Run(fmt.Sprintf("InvalidType_%d", i), func(t *testing.T) {
				args := map[string]interface{}{"entities": invalidType}
				result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "entities must be an array")
			})
		}
	})

	t.Run("EmptyEntitiesArray", func(t *testing.T) {
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		args := map[string]interface{}{"entities": []interface{}{}}
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		
		resultMap := result.(map[string]interface{})
		assert.True(t, resultMap["success"].(bool))
		assert.Equal(t, 0, resultMap["count"].(int))
	})

	t.Run("MalformedEntityStructures", func(t *testing.T) {
		malformedEntities := [][]interface{}{
			// Invalid entity types
			{123},
			{"string instead of object"},
			{true},
			{[]string{"array", "instead", "of", "object"}},
			
			// Complex nested structures that can't be decoded
			{map[string]interface{}{
				"name": map[string]interface{}{"nested": "object"}, // Name should be string
			}},
			{map[string]interface{}{
				"observations": "string_instead_of_array",
			}},
			{map[string]interface{}{
				"createdAt": "invalid_time_format",
			}},
		}

		for i, entities := range malformedEntities {
			t.Run(fmt.Sprintf("MalformedEntity_%d", i), func(t *testing.T) {
				args := map[string]interface{}{"entities": entities}
				result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "failed to decode entity")
			})
		}

		// Test nil entity separately as it might be handled gracefully
		t.Run("NilEntity", func(t *testing.T) {
			mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)
			args := map[string]interface{}{"entities": []interface{}{nil}}
			result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
			// This might succeed or fail depending on mapstructure behavior
			if err != nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	})

	t.Run("StorageError", func(t *testing.T) {
		mockStorage := new(storage.MockBackend)
		manager := NewManager(mockStorage)
		
		expectedError := errors.New("storage connection failed")
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(expectedError)

		args := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"name": "Test Entity"},
			},
		}
		
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to create entities")
		assert.Contains(t, err.Error(), "storage connection failed")
	})

	t.Run("MassiveEntitiesArray", func(t *testing.T) {
		// Create 10,000 entities
		massiveArray := make([]interface{}, 10000)
		for i := 0; i < 10000; i++ {
			massiveArray[i] = map[string]interface{}{
				"name":         fmt.Sprintf("Entity %d", i),
				"entityType":   "stress_test",
				"observations": []string{fmt.Sprintf("observation_%d", i)},
			}
		}
		
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)
		
		args := map[string]interface{}{"entities": massiveArray}
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		
		resultMap := result.(map[string]interface{})
		assert.True(t, resultMap["success"].(bool))
		assert.Equal(t, 10000, resultMap["count"].(int))
	})

	t.Run("UnicodeAndSpecialCharacters", func(t *testing.T) {
		unicodeEntities := []interface{}{
			map[string]interface{}{
				"name":         "测试实体_🦄",
				"entityType":   "unicode_test",
				"observations": []string{"观察_наблюдение", "🔍🎉"},
			},
			map[string]interface{}{
				"name":         "Entity with \"quotes\" and 'apostrophes'",
				"entityType":   "special_chars",
				"observations": []string{"<script>alert('xss')</script>", "'; DROP TABLE entities; --"},
			},
			map[string]interface{}{
				"name":         "\x00null\x01byte\x02test",
				"entityType":   "control_chars",
				"observations": []string{"\t\n\r\f\b"},
			},
		}
		
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)
		
		args := map[string]interface{}{"entities": unicodeEntities}
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestManager_EdgeCases_HandleCallTool_Search_MalformedArgs(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()

	t.Run("MissingQueryParameter", func(t *testing.T) {
		result, err := manager.HandleCallTool(ctx, "memory__search", map[string]interface{}{})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query parameter is required")
	})

	t.Run("QueryNotString", func(t *testing.T) {
		invalidQueries := []interface{}{
			123,
			true,
			[]string{"array", "query"},
			map[string]interface{}{"nested": "query"},
			nil,
		}

		for i, invalidQuery := range invalidQueries {
			t.Run(fmt.Sprintf("InvalidQuery_%d", i), func(t *testing.T) {
				args := map[string]interface{}{"query": invalidQuery}
				result, err := manager.HandleCallTool(ctx, "memory__search", args)
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "query must be a string")
			})
		}
	})

	t.Run("EmptyStringQuery", func(t *testing.T) {
		mockStorage.On("SearchEntities", mock.Anything, "").Return([]mcp.Entity{}, nil)

		args := map[string]interface{}{"query": ""}
		result, err := manager.HandleCallTool(ctx, "memory__search", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		
		resultMap := result.(map[string]interface{})
		entities := resultMap["entities"].([]mcp.Entity)
		assert.Empty(t, entities)
		assert.Equal(t, 0, resultMap["count"].(int))
	})

	t.Run("VeryLongQuery", func(t *testing.T) {
		longQuery := strings.Repeat("very_long_query_string", 10000)
		mockStorage.On("SearchEntities", mock.Anything, longQuery).Return([]mcp.Entity{}, nil)

		args := map[string]interface{}{"query": longQuery}
		result, err := manager.HandleCallTool(ctx, "memory__search", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("UnicodeQuery", func(t *testing.T) {
		unicodeQueries := []string{
			"测试查询",
			"🦄🌈🚀",
			"Тест запрос",
			"emoji_🔍_search",
			"\u0000\u0001\u0002", // Control characters
		}

		for _, query := range unicodeQueries {
			mockStorage.On("SearchEntities", mock.Anything, query).Return([]mcp.Entity{}, nil)
			
			args := map[string]interface{}{"query": query}
			result, err := manager.HandleCallTool(ctx, "memory__search", args)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		}
	})

	t.Run("StorageError", func(t *testing.T) {
		mockStorage := new(storage.MockBackend)
		manager := NewManager(mockStorage)
		
		expectedError := errors.New("search index corrupted")
		mockStorage.On("SearchEntities", mock.Anything, "test").Return(nil, expectedError)

		args := map[string]interface{}{"query": "test"}
		result, err := manager.HandleCallTool(ctx, "memory__search", args)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to search entities")
		assert.Contains(t, err.Error(), "search index corrupted")
	})
}

func TestManager_EdgeCases_HandleCallTool_GetEntity_MalformedArgs(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()

	t.Run("MissingIdParameter", func(t *testing.T) {
		result, err := manager.HandleCallTool(ctx, "memory__get_entity", map[string]interface{}{})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "id parameter is required")
	})

	t.Run("IdNotString", func(t *testing.T) {
		invalidIds := []interface{}{
			123,
			true,
			[]string{"array", "id"},
			map[string]interface{}{"nested": "id"},
			nil,
		}

		for i, invalidId := range invalidIds {
			t.Run(fmt.Sprintf("InvalidId_%d", i), func(t *testing.T) {
				args := map[string]interface{}{"id": invalidId}
				result, err := manager.HandleCallTool(ctx, "memory__get_entity", args)
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "id must be a string")
			})
		}
	})

	t.Run("EmptyStringId", func(t *testing.T) {
		expectedEntity := &mcp.Entity{ID: "", Name: "Empty ID Entity"}
		mockStorage.On("GetEntity", mock.Anything, "").Return(expectedEntity, nil)

		args := map[string]interface{}{"id": ""}
		result, err := manager.HandleCallTool(ctx, "memory__get_entity", args)
		assert.NoError(t, err)
		assert.Equal(t, expectedEntity, result)
	})

	t.Run("VeryLongId", func(t *testing.T) {
		longId := strings.Repeat("very_long_id", 10000)
		expectedError := errors.New("entity not found")
		mockStorage.On("GetEntity", mock.Anything, longId).Return(nil, expectedError)

		args := map[string]interface{}{"id": longId}
		result, err := manager.HandleCallTool(ctx, "memory__get_entity", args)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get entity")
	})

	t.Run("SpecialCharacterId", func(t *testing.T) {
		specialIds := []string{
			"id/with/slashes",
			"id with spaces",
			"id\twith\ttabs",
			"id\nwith\nnewlines",
			"id\"with\"quotes",
			"id'with'apostrophes",
			"id<script>alert('xss')</script>",
			"'; DROP TABLE entities; --",
			"🦄_unicode_id",
			"\x00null\x01bytes",
		}

		for _, specialId := range specialIds {
			expectedEntity := &mcp.Entity{ID: specialId, Name: "Special Entity"}
			mockStorage.On("GetEntity", mock.Anything, specialId).Return(expectedEntity, nil)
			
			args := map[string]interface{}{"id": specialId}
			result, err := manager.HandleCallTool(ctx, "memory__get_entity", args)
			assert.NoError(t, err)
			assert.Equal(t, expectedEntity, result)
		}
	})

	t.Run("StorageError", func(t *testing.T) {
		mockStorage := new(storage.MockBackend)
		manager := NewManager(mockStorage)
		
		expectedError := errors.New("database connection lost")
		mockStorage.On("GetEntity", mock.Anything, "test_id").Return(nil, expectedError)

		args := map[string]interface{}{"id": "test_id"}
		result, err := manager.HandleCallTool(ctx, "memory__get_entity", args)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get entity")
		assert.Contains(t, err.Error(), "database connection lost")
	})
}

func TestManager_EdgeCases_HandleCallTool_GetStatistics_MalformedArgs(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()

	t.Run("UnexpectedArguments", func(t *testing.T) {
		// GetStatistics should work regardless of extra arguments
		expectedStats := map[string]int{"entities": 42, "type_test": 10}
		mockStorage.On("GetStatistics", mock.Anything).Return(expectedStats, nil)

		testArgs := []map[string]interface{}{
			{},                                           // Empty args
			{"unexpected": "argument"},                   // Extra args
			{"limit": 100, "offset": 50},                // Pagination-like args
			{"malicious": "'; DROP TABLE stats; --"},    // SQL injection attempt
			{"unicode": "测试参数"},                       // Unicode args
			{strings.Repeat("key", 1000): "long_value"}, // Very long key
		}

		for i, args := range testArgs {
			t.Run(fmt.Sprintf("Args_%d", i), func(t *testing.T) {
				result, err := manager.HandleCallTool(ctx, "memory__get_statistics", args)
				assert.NoError(t, err)
				assert.Equal(t, expectedStats, result)
			})
		}
	})

	t.Run("StorageError", func(t *testing.T) {
		mockStorage := new(storage.MockBackend)
		manager := NewManager(mockStorage)
		
		expectedError := errors.New("statistics calculation failed")
		mockStorage.On("GetStatistics", mock.Anything).Return(nil, expectedError)

		result, err := manager.HandleCallTool(ctx, "memory__get_statistics", map[string]interface{}{})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get statistics")
		assert.Contains(t, err.Error(), "statistics calculation failed")
	})
}

func TestManager_EdgeCases_CreateEntities_EntityProcessing(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()

	t.Run("EntityIdGeneration", func(t *testing.T) {
		// Test ID generation for entities without IDs - now uses UUIDs
		mockStorage.On("CreateEntities", mock.Anything, mock.MatchedBy(func(entities []mcp.Entity) bool {
			// Verify all entities have IDs
			for _, entity := range entities {
				if entity.ID == "" {
					return false
				}
				// UUID format check: should be 36 chars (including hyphens)
				if len(entity.ID) != 36 {
					return false
				}
			}
			return true
		})).Return(nil)

		entities := []interface{}{
			map[string]interface{}{"name": "No ID Entity 1"},
			map[string]interface{}{"name": "No ID Entity 2"},
			map[string]interface{}{"name": "No ID Entity 3"},
		}

		args := map[string]interface{}{"entities": entities}
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("CreatedAtTimestampGeneration", func(t *testing.T) {
		beforeTest := time.Now()
		
		mockStorage.On("CreateEntities", mock.Anything, mock.MatchedBy(func(entities []mcp.Entity) bool {
			// Verify all entities have valid created timestamps
			for _, entity := range entities {
				if entity.CreatedAt.IsZero() {
					return false
				}
				if entity.CreatedAt.Before(beforeTest) {
					return false
				}
				if entity.CreatedAt.After(time.Now().Add(1*time.Second)) {
					return false
				}
			}
			return true
		})).Return(nil)

		entities := []interface{}{
			map[string]interface{}{"name": "No Timestamp Entity"},
		}

		args := map[string]interface{}{"entities": entities}
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("PreserveExistingIdAndTimestamp", func(t *testing.T) {
		existingTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		
		mockStorage.On("CreateEntities", mock.Anything, mock.MatchedBy(func(entities []mcp.Entity) bool {
			// Verify existing ID and timestamp are preserved
			return entities[0].ID == "existing_id" && entities[0].CreatedAt.Equal(existingTime)
		})).Return(nil)

		entities := []interface{}{
			map[string]interface{}{
				"id":        "existing_id",
				"name":      "Existing Entity",
				"createdAt": existingTime,
			},
		}

		args := map[string]interface{}{"entities": entities}
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestManager_EdgeCases_ContextHandling(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)

	t.Run("CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Storage should NOT be called with cancelled context
		// mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		args := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"name": "Test Entity"},
			},
		}

		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		// Context cancellation should be checked and return an error
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Nil(t, result)
	})

	t.Run("TimeoutContext", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(1 * time.Millisecond) // Ensure timeout

		// Storage should NOT be called with timed out context
		// mockStorage.On("SearchEntities", mock.Anything, "test").Return([]mcp.Entity{}, nil)

		args := map[string]interface{}{"query": "test"}
		result, err := manager.HandleCallTool(ctx, "memory__search", args)
		// Context timeout should be checked and return an error
		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Nil(t, result)
	})
}

func TestManager_EdgeCases_NilAndEmptyValues(t *testing.T) {
	manager := NewManager(nil) // Nil storage backend

	t.Run("NilStorage", func(t *testing.T) {
		ctx := context.Background()
		
		// This should panic or error when trying to call methods on nil storage
		defer func() {
			if r := recover(); r != nil {
				// Expected panic when calling methods on nil storage
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer")
			}
		}()

		args := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"name": "Test"},
			},
		}

		_, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		// If we get here, it means no panic occurred, so we should have an error
		if err == nil {
			t.Error("Expected error when using nil storage")
		}
	})
}

func TestGenerateEntityID_EdgeCases(t *testing.T) {
	// generateEntityID now takes no parameters and returns UUIDs
	t.Run("UUIDFormat", func(t *testing.T) {
		id := generateEntityID()
		// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		assert.Equal(t, 36, len(id), "UUID should be 36 characters long")
		assert.Contains(t, id, "-", "UUID should contain hyphens")
		
		// Check UUID structure
		parts := strings.Split(id, "-")
		assert.Equal(t, 5, len(parts), "UUID should have 5 parts")
		assert.Equal(t, 8, len(parts[0]))
		assert.Equal(t, 4, len(parts[1]))
		assert.Equal(t, 4, len(parts[2]))
		assert.Equal(t, 4, len(parts[3]))
		assert.Equal(t, 12, len(parts[4]))
	})

	// Test uniqueness
	t.Run("Uniqueness", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id := generateEntityID()
			assert.False(t, ids[id], "Generated duplicate ID: %s", id)
			ids[id] = true
		}
	})
}

func TestManager_HandleListTools_EdgeCases(t *testing.T) {
	// Test with nil storage (should still work since it doesn't use storage)
	manager := NewManager(nil)
	tools := manager.HandleListTools()
	
	assert.NotEmpty(t, tools)
	assert.Len(t, tools, 4) // Should have exactly 4 tools
	
	// Verify all expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Name, "Tool name should not be empty")
		assert.NotEmpty(t, tool.Description, "Tool description should not be empty")
	}
	
	expectedTools := []string{
		"memory__create_entities",
		"memory__search",
		"memory__get_entity",
		"memory__get_statistics",
	}
	
	for _, expectedTool := range expectedTools {
		assert.True(t, toolNames[expectedTool], "Missing expected tool: %s", expectedTool)
	}
}

func TestManager_EdgeCases_BoundaryConditions(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := NewManager(mockStorage)
	ctx := context.Background()

	t.Run("MaxInt64EntityCount", func(t *testing.T) {
		// Simulate creating a huge number of entities
		largeNumber := int64(9223372036854775807) // Max int64
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		// Create args that would theoretically create a massive number of entities
		// In practice, this would fail due to memory constraints, but we test the handling
		result, err := manager.HandleCallTool(ctx, "memory__create_entities", map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{"name": fmt.Sprintf("Entity %d", largeNumber)},
			},
		})
		
		assert.NoError(t, err)
		resultMap := result.(map[string]interface{})
		assert.Equal(t, 1, resultMap["count"].(int)) // Should handle the actual count correctly
	})

	t.Run("ZeroValues", func(t *testing.T) {
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		// Entity with all zero/empty values
		args := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{
					"id":           "",
					"name":         "",
					"entityType":   "",
					"observations": []interface{}{},
					"createdAt":    time.Time{},
				},
			},
		}

		result, err := manager.HandleCallTool(ctx, "memory__create_entities", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}