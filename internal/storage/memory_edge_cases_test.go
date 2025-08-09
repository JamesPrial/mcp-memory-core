package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBackend_EdgeCases_CreateEntities(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	t.Run("EmptyEntitiesSlice", func(t *testing.T) {
		err := backend.CreateEntities(ctx, []mcp.Entity{})
		assert.NoError(t, err)
	})

	t.Run("NilEntitiesSlice", func(t *testing.T) {
		err := backend.CreateEntities(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("VeryLargeEntityName", func(t *testing.T) {
		largeString := strings.Repeat("a", 1000000) // 1MB string
		entity := mcp.Entity{
			ID:           "large_name",
			Name:         largeString,
			EntityType:   "test",
			Observations: []string{"obs1"},
			CreatedAt:    time.Now(),
		}
		
		err := backend.CreateEntities(ctx, []mcp.Entity{entity})
		assert.NoError(t, err)
		
		// Verify it was stored
		retrieved, err := backend.GetEntity(ctx, "large_name")
		require.NoError(t, err)
		assert.Equal(t, largeString, retrieved.Name)
	})

	t.Run("UnicodeAndSpecialCharacters", func(t *testing.T) {
		unicodeEntity := mcp.Entity{
			ID:           "unicode_test_🦄",
			Name:         "测试名称_Тест_🦄_♻️_™",
			EntityType:   "unicode_type_🚀",
			Observations: []string{"观察_наблюдение_🔍", "emoji_test_🎉🎊"},
			CreatedAt:    time.Now(),
		}
		
		err := backend.CreateEntities(ctx, []mcp.Entity{unicodeEntity})
		assert.NoError(t, err)
		
		// Verify it was stored with unicode intact
		retrieved, err := backend.GetEntity(ctx, "unicode_test_🦄")
		require.NoError(t, err)
		assert.Equal(t, "测试名称_Тест_🦄_♻️_™", retrieved.Name)
		assert.Equal(t, "unicode_type_🚀", retrieved.EntityType)
		assert.Contains(t, retrieved.Observations, "观察_наблюдение_🔍")
	})

	t.Run("MassiveObservationsArray", func(t *testing.T) {
		// Create entity with 10,000 observations
		observations := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			observations[i] = fmt.Sprintf("observation_%d_with_some_content", i)
		}
		
		entity := mcp.Entity{
			ID:           "massive_obs",
			Name:         "Entity with massive observations",
			EntityType:   "stress_test",
			Observations: observations,
			CreatedAt:    time.Now(),
		}
		
		err := backend.CreateEntities(ctx, []mcp.Entity{entity})
		assert.NoError(t, err)
		
		// Verify all observations are preserved
		retrieved, err := backend.GetEntity(ctx, "massive_obs")
		require.NoError(t, err)
		assert.Len(t, retrieved.Observations, 10000)
		assert.Equal(t, "observation_5000_with_some_content", retrieved.Observations[5000])
	})

	t.Run("EmptyStringFields", func(t *testing.T) {
		entity := mcp.Entity{
			ID:           "",
			Name:         "",
			EntityType:   "",
			Observations: []string{"", "   ", "\t\n"},
			CreatedAt:    time.Time{},
		}
		
		err := backend.CreateEntities(ctx, []mcp.Entity{entity})
		assert.NoError(t, err)
		
		// Should be able to retrieve by empty ID
		retrieved, err := backend.GetEntity(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, "", retrieved.Name)
		assert.Equal(t, "", retrieved.EntityType)
		assert.Len(t, retrieved.Observations, 3)
	})
}

func TestMemoryBackend_EdgeCases_GetEntity(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	t.Run("NonExistentID", func(t *testing.T) {
		entity, err := backend.GetEntity(ctx, "non_existent")
		assert.Error(t, err)
		assert.Nil(t, entity)
		assert.Contains(t, err.Error(), "entity not found")
	})

	t.Run("EmptyStringID", func(t *testing.T) {
		// First create entity with empty ID
		entity := mcp.Entity{ID: "", Name: "empty id entity"}
		err := backend.CreateEntities(ctx, []mcp.Entity{entity})
		require.NoError(t, err)
		
		// Should be retrievable
		retrieved, err := backend.GetEntity(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, "empty id entity", retrieved.Name)
	})

	t.Run("SpecialCharacterIDs", func(t *testing.T) {
		specialIDs := []string{
			"id-with-dashes",
			"id_with_underscores",
			"id.with.dots",
			"id with spaces",
			"id\twith\ttabs",
			"id\nwith\nnewlines",
			"id🦄with🚀emoji",
			"id/with/slashes",
			"id\\with\\backslashes",
			"id\"with\"quotes",
			"id'with'single'quotes",
			"id(with)parentheses",
			"id[with]brackets",
			"id{with}braces",
			"id<with>angles",
		}
		
		// Create entities with special character IDs
		entities := make([]mcp.Entity, len(specialIDs))
		for i, id := range specialIDs {
			entities[i] = mcp.Entity{
				ID:   id,
				Name: fmt.Sprintf("Entity with special ID: %s", id),
			}
		}
		
		err := backend.CreateEntities(ctx, entities)
		require.NoError(t, err)
		
		// Retrieve each one
		for _, id := range specialIDs {
			retrieved, err := backend.GetEntity(ctx, id)
			require.NoError(t, err, "Failed to retrieve entity with ID: %s", id)
			assert.Equal(t, id, retrieved.ID)
		}
	})
}

func TestMemoryBackend_EdgeCases_SearchEntities(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Setup test data
	entities := []mcp.Entity{
		{ID: "1", Name: "John Doe", EntityType: "person", Observations: []string{"Works at ACME", "Likes coffee"}},
		{ID: "2", Name: "Jane Smith", EntityType: "person", Observations: []string{"CEO of TechCorp", "Plays tennis"}},
		{ID: "3", Name: "测试用户", EntityType: "person", Observations: []string{"中文观察", "English observation"}},
		{ID: "4", Name: "🦄 Unicorn Entity", EntityType: "mythical", Observations: []string{"🌈 Rainbow maker", "🦄 Magic user"}},
	}
	err := backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	t.Run("EmptyQuery", func(t *testing.T) {
		results, err := backend.SearchEntities(ctx, "")
		assert.NoError(t, err)
		assert.Len(t, results, 4) // Should return all entities when query is empty
	})

	t.Run("WhitespaceOnlyQuery", func(t *testing.T) {
		results, err := backend.SearchEntities(ctx, "   \t\n   ")
		assert.NoError(t, err)
		assert.Len(t, results, 0) // Should return no results for whitespace-only query
	})

	t.Run("VeryLongQuery", func(t *testing.T) {
		longQuery := strings.Repeat("superlongquerythatdoesntexist", 1000)
		results, err := backend.SearchEntities(ctx, longQuery)
		assert.NoError(t, err)
		assert.Len(t, results, 0)
	})

	t.Run("UnicodeQuery", func(t *testing.T) {
		results, err := backend.SearchEntities(ctx, "测试")
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "测试用户", results[0].Name)
		
		results, err = backend.SearchEntities(ctx, "中文")
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "测试用户", results[0].Name)
	})

	t.Run("EmojiQuery", func(t *testing.T) {
		results, err := backend.SearchEntities(ctx, "🦄")
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "🦄 Unicorn Entity", results[0].Name)
		
		results, err = backend.SearchEntities(ctx, "🌈")
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Observations, "🌈 Rainbow maker")
	})

	t.Run("SpecialCharacterQuery", func(t *testing.T) {
		specialQueries := []string{
			"@#$%^&*()",
			"<script>alert('xss')</script>",
			"'; DROP TABLE entities; --",
			"\\x00\\x01\\x02",
			"\u0000\u0001\u0002",
		}
		
		for _, query := range specialQueries {
			results, err := backend.SearchEntities(ctx, query)
			assert.NoError(t, err, "Query should not error: %s", query)
			assert.Len(t, results, 0, "Should return no results for: %s", query)
		}
	})

	t.Run("CaseInsensitiveSearch", func(t *testing.T) {
		// Test case insensitivity
		testCases := []struct {
			query         string
			expectedCount int
		}{
			{"john", 1},
			{"JOHN", 1},
			{"JoHn", 1},
			{"doe", 1},
			{"DOE", 1},
			{"acme", 1},
			{"ACME", 1},
		}
		
		for _, tc := range testCases {
			results, err := backend.SearchEntities(ctx, tc.query)
			assert.NoError(t, err)
			assert.Len(t, results, tc.expectedCount, "Query: %s", tc.query)
		}
	})
}

func TestMemoryBackend_EdgeCases_GetStatistics(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	t.Run("EmptyStorage", func(t *testing.T) {
		stats, err := backend.GetStatistics(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, stats["entities"])
		assert.NotContains(t, stats, "type_")
	})

	t.Run("EntitiesWithoutType", func(t *testing.T) {
		entities := []mcp.Entity{
			{ID: "1", Name: "No Type 1", EntityType: ""},
			{ID: "2", Name: "No Type 2", EntityType: ""},
		}
		err := backend.CreateEntities(ctx, entities)
		require.NoError(t, err)
		
		stats, err := backend.GetStatistics(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 2, stats["entities"])
		assert.NotContains(t, stats, "type_")
	})

	t.Run("MixedEntityTypes", func(t *testing.T) {
		backend := NewMemoryBackend() // Fresh backend
		entities := []mcp.Entity{
			{ID: "1", Name: "Person 1", EntityType: "person"},
			{ID: "2", Name: "Person 2", EntityType: "person"},
			{ID: "3", Name: "Company 1", EntityType: "company"},
			{ID: "4", Name: "No Type", EntityType: ""},
			{ID: "5", Name: "Unicode Type", EntityType: "类型_🚀"},
		}
		err := backend.CreateEntities(ctx, entities)
		require.NoError(t, err)
		
		stats, err := backend.GetStatistics(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 5, stats["entities"])
		assert.Equal(t, 2, stats["type_person"])
		assert.Equal(t, 1, stats["type_company"])
		assert.Equal(t, 1, stats["type_类型_🚀"])
		assert.NotContains(t, stats, "type_")
	})

	t.Run("MassiveNumberOfEntities", func(t *testing.T) {
		backend := NewMemoryBackend() // Fresh backend
		entities := make([]mcp.Entity, 10000)
		for i := 0; i < 10000; i++ {
			entities[i] = mcp.Entity{
				ID:         fmt.Sprintf("entity_%d", i),
				Name:       fmt.Sprintf("Entity %d", i),
				EntityType: fmt.Sprintf("type_%d", i%100), // 100 different types
			}
		}
		
		err := backend.CreateEntities(ctx, entities)
		require.NoError(t, err)
		
		stats, err := backend.GetStatistics(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 10000, stats["entities"])
		
		// Should have 100 different types, each with 100 entities
		typeCount := 0
		for key, value := range stats {
			if strings.HasPrefix(key, "type_") {
				typeCount++
				assert.Equal(t, 100, value, "Each type should have 100 entities")
			}
		}
		assert.Equal(t, 100, typeCount)
	})
}

func TestMemoryBackend_EdgeCases_ConcurrentAccess(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	t.Run("ConcurrentCreateAndRead", func(t *testing.T) {
		const numGoroutines = 100
		const entitiesPerGoroutine = 10
		
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		// Concurrent writes
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				entities := make([]mcp.Entity, entitiesPerGoroutine)
				for j := 0; j < entitiesPerGoroutine; j++ {
					entities[j] = mcp.Entity{
						ID:         fmt.Sprintf("g%d_e%d", goroutineID, j),
						Name:       fmt.Sprintf("Goroutine %d Entity %d", goroutineID, j),
						EntityType: "concurrent",
					}
				}
				err := backend.CreateEntities(ctx, entities)
				assert.NoError(t, err)
			}(i)
		}
		
		wg.Wait()
		
		// Verify all entities were created
		stats, err := backend.GetStatistics(ctx)
		require.NoError(t, err)
		assert.Equal(t, numGoroutines*entitiesPerGoroutine, stats["entities"])
	})

	t.Run("ConcurrentReadWhileWriting", func(t *testing.T) {
		// Pre-populate with some entities
		baseEntities := make([]mcp.Entity, 100)
		for i := 0; i < 100; i++ {
			baseEntities[i] = mcp.Entity{
				ID:   fmt.Sprintf("base_%d", i),
				Name: fmt.Sprintf("Base Entity %d", i),
			}
		}
		err := backend.CreateEntities(ctx, baseEntities)
		require.NoError(t, err)
		
		var wg sync.WaitGroup
		const numReaders = 50
		const numWriters = 10
		const duration = 100 * time.Millisecond
		
		// Start readers
		wg.Add(numReaders)
		for i := 0; i < numReaders; i++ {
			go func(readerID int) {
				defer wg.Done()
				end := time.Now().Add(duration)
				for time.Now().Before(end) {
					// Random reads
					entityID := fmt.Sprintf("base_%d", readerID%100)
					entity, err := backend.GetEntity(ctx, entityID)
					if err == nil {
						assert.NotEmpty(t, entity.Name)
					}
					
					// Search operations
					results, err := backend.SearchEntities(ctx, "Base")
					assert.NoError(t, err)
					assert.NotEmpty(t, results)
				}
			}(i)
		}
		
		// Start writers
		wg.Add(numWriters)
		for i := 0; i < numWriters; i++ {
			go func(writerID int) {
				defer wg.Done()
				end := time.Now().Add(duration)
				counter := 0
				for time.Now().Before(end) {
					entity := mcp.Entity{
						ID:   fmt.Sprintf("writer_%d_%d", writerID, counter),
						Name: fmt.Sprintf("Writer %d Entity %d", writerID, counter),
					}
					err := backend.CreateEntities(ctx, []mcp.Entity{entity})
					assert.NoError(t, err)
					counter++
				}
			}(i)
		}
		
		wg.Wait()
		
		// Verify no corruption occurred
		stats, err := backend.GetStatistics(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, stats["entities"], 100) // At least the base entities
	})

	t.Run("ConcurrentSearchOperations", func(t *testing.T) {
		// Setup diverse test data
		setupEntities := []mcp.Entity{
			{ID: "search_1", Name: "Apple Product", EntityType: "product", Observations: []string{"iPhone", "Technology"}},
			{ID: "search_2", Name: "Orange Fruit", EntityType: "food", Observations: []string{"Citrus", "Vitamin C"}},
			{ID: "search_3", Name: "Banana Split", EntityType: "dessert", Observations: []string{"Ice cream", "Sweet"}},
		}
		err := backend.CreateEntities(ctx, setupEntities)
		require.NoError(t, err)
		
		var wg sync.WaitGroup
		const numSearchers = 100
		queries := []string{"Apple", "Orange", "Banana", "Product", "Fruit", "Sweet", "Technology"}
		
		wg.Add(numSearchers)
		for i := 0; i < numSearchers; i++ {
			go func(searcherID int) {
				defer wg.Done()
				query := queries[searcherID%len(queries)]
				results, err := backend.SearchEntities(ctx, query)
				assert.NoError(t, err)
				// Should find at least some results for valid queries
				if query == "Apple" || query == "Orange" || query == "Banana" {
					assert.NotEmpty(t, results)
				}
			}(i)
		}
		
		wg.Wait()
	})
}

func TestMemoryBackend_EdgeCases_ContextCancellation(t *testing.T) {
	backend := NewMemoryBackend()

	t.Run("CancelledContextOnCreate", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		entity := mcp.Entity{ID: "cancelled", Name: "Should not be created"}
		err := backend.CreateEntities(ctx, []mcp.Entity{entity})
		// Memory backend doesn't check context cancellation, so this will succeed
		// but in a real implementation, you might want to check ctx.Done()
		assert.NoError(t, err)
	})

	t.Run("TimeoutContext", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(1 * time.Millisecond) // Ensure timeout
		
		entity := mcp.Entity{ID: "timeout", Name: "Should timeout"}
		err := backend.CreateEntities(ctx, []mcp.Entity{entity})
		// Memory backend doesn't check context timeout, so this will succeed
		assert.NoError(t, err)
	})
}

func TestMemoryBackend_EdgeCases_MemoryPressure(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	t.Run("LargeDataStructures", func(t *testing.T) {
		// Create entities with very large observation arrays and names
		largeEntities := make([]mcp.Entity, 10)
		for i := 0; i < 10; i++ {
			// Each observation is ~1KB, 1000 observations = ~1MB per entity
			observations := make([]string, 1000)
			for j := 0; j < 1000; j++ {
				observations[j] = strings.Repeat(fmt.Sprintf("obs_%d_%d_", i, j), 50)
			}
			
			largeEntities[i] = mcp.Entity{
				ID:           fmt.Sprintf("large_entity_%d", i),
				Name:         strings.Repeat(fmt.Sprintf("large_name_%d_", i), 100),
				EntityType:   "memory_pressure_test",
				Observations: observations,
				CreatedAt:    time.Now(),
			}
		}
		
		err := backend.CreateEntities(ctx, largeEntities)
		assert.NoError(t, err)
		
		// Verify data integrity
		for i := 0; i < 10; i++ {
			entity, err := backend.GetEntity(ctx, fmt.Sprintf("large_entity_%d", i))
			require.NoError(t, err)
			assert.Len(t, entity.Observations, 1000)
			assert.Contains(t, entity.Name, fmt.Sprintf("large_name_%d_", i))
		}
		
		// Search should still work (search by entity name prefix)
		results, err := backend.SearchEntities(ctx, "large_name")
		assert.NoError(t, err)
		assert.Len(t, results, 10)
	})
}

func TestContainsIgnoreCase_EdgeCases(t *testing.T) {
	testCases := []struct {
		str      string
		substr   string
		expected bool
	}{
		// Empty strings
		{"", "", true},
		{"", "a", false},
		{"a", "", true},
		
		// Unicode and special characters
		{"测试字符串", "测试", true},
		{"ТЕСТ", "тест", true},
		{"🦄🌈🚀", "🌈", true},
		{"🦄🌈🚀", "🌟", false},
		
		// Mixed case with special chars
		{"Hello-World_123", "HELLO", true},
		{"Hello-World_123", "world", true},
		{"Hello-World_123", "123", true},
		
		// Whitespace
		{"  spaced  out  ", "spaced", true},
		{"  spaced  out  ", "SPACED", true},
		{"\t\n\r", "\t", true},
		
		// Long strings
		{strings.Repeat("a", 10000), "a", true},
		{strings.Repeat("a", 10000), "b", false},
		{strings.Repeat("ab", 5000), "ab", true},
	}
	
	for _, tc := range testCases {
		result := containsIgnoreCase(tc.str, tc.substr)
		assert.Equal(t, tc.expected, result, 
			"containsIgnoreCase(%q, %q) = %v, want %v", 
			tc.str, tc.substr, result, tc.expected)
	}
}