package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBackend_NewMemoryBackend(t *testing.T) {
	backend := NewMemoryBackend()
	require.NotNil(t, backend)
	assert.NotNil(t, backend.entities)
}

func TestMemoryBackend_CreateEntities(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	entity := mcp.Entity{
		ID:           "test-1",
		Name:         "Test Entity",
		EntityType:   mcp.EntityTypePerson,
		Observations: []string{"observation 1", "observation 2"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := backend.CreateEntities(ctx, []mcp.Entity{entity})
	require.NoError(t, err)

	// Verify entity was stored
	retrieved, err := backend.GetEntity(ctx, "test-1")
	require.NoError(t, err)
	assert.Equal(t, entity.ID, retrieved.ID)
	assert.Equal(t, entity.Name, retrieved.Name)
	assert.Equal(t, entity.EntityType, retrieved.EntityType)
	assert.Equal(t, entity.Observations, retrieved.Observations)
}

func TestMemoryBackend_CreateMultipleEntities(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	entities := []mcp.Entity{
		{
			ID:         "entity-1",
			Name:       "First Entity",
			EntityType: mcp.EntityTypePerson,
			CreatedAt:  time.Now(),
		},
		{
			ID:         "entity-2",
			Name:       "Second Entity",
			EntityType: mcp.EntityTypeOrganization,
			CreatedAt:  time.Now(),
		},
	}

	err := backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	// Verify both entities were stored
	for _, entity := range entities {
		retrieved, err := backend.GetEntity(ctx, entity.ID)
		require.NoError(t, err)
		assert.Equal(t, entity.ID, retrieved.ID)
		assert.Equal(t, entity.Name, retrieved.Name)
	}
}

func TestMemoryBackend_GetEntity_NotFound(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	entity, err := backend.GetEntity(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, entity)
}

func TestMemoryBackend_SearchEntities(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Create test entities
	entities := []mcp.Entity{
		{
			ID:           "search-1",
			Name:         "John Doe",
			EntityType:   mcp.EntityTypePerson,
			Observations: []string{"Works at ACME Corp", "Lives in Seattle"},
			CreatedAt:    time.Now(),
		},
		{
			ID:           "search-2",
			Name:         "Jane Smith",
			EntityType:   mcp.EntityTypePerson,
			Observations: []string{"CEO of TechStart", "From Portland"},
			CreatedAt:    time.Now(),
		},
		{
			ID:           "search-3",
			Name:         "ACME Corporation",
			EntityType:   mcp.EntityTypeOrganization,
			Observations: []string{"Technology company", "Founded in 1990"},
			CreatedAt:    time.Now(),
		},
	}

	err := backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	tests := []struct {
		name          string
		query         string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "search by name",
			query:         "John",
			expectedCount: 1,
			expectedIDs:   []string{"search-1"},
		},
		{
			name:          "search by observation",
			query:         "ACME",
			expectedCount: 2, // Both John (works at ACME) and ACME Corp itself
			expectedIDs:   []string{"search-1", "search-3"},
		},
		{
			name:          "case insensitive search",
			query:         "seattle",
			expectedCount: 1,
			expectedIDs:   []string{"search-1"},
		},
		{
			name:          "no results",
			query:         "nonexistent",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := backend.SearchEntities(ctx, tt.query)
			require.NoError(t, err)
			assert.Len(t, results, tt.expectedCount)

			if len(tt.expectedIDs) > 0 {
				resultIDs := make([]string, len(results))
				for i, result := range results {
					resultIDs[i] = result.ID
				}
				for _, expectedID := range tt.expectedIDs {
					assert.Contains(t, resultIDs, expectedID)
				}
			}
		})
	}
}

func TestMemoryBackend_GetStatistics(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Test empty backend
	stats, err := backend.GetStatistics(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats["entities"])

	// Add entities of different types
	entities := []mcp.Entity{
		{ID: "stat-1", Name: "Person 1", EntityType: mcp.EntityTypePerson, CreatedAt: time.Now()},
		{ID: "stat-2", Name: "Person 2", EntityType: mcp.EntityTypePerson, CreatedAt: time.Now()},
		{ID: "stat-3", Name: "Org 1", EntityType: mcp.EntityTypeOrganization, CreatedAt: time.Now()},
		{ID: "stat-4", Name: "Project 1", EntityType: mcp.EntityTypeProject, CreatedAt: time.Now()},
	}

	err = backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	stats, err = backend.GetStatistics(ctx)
	require.NoError(t, err)
	
	assert.Equal(t, 4, stats["entities"])
	assert.Equal(t, 2, stats["type_Person"])
	assert.Equal(t, 1, stats["type_Organization"])
	assert.Equal(t, 1, stats["type_Project"])
}

func TestMemoryBackend_Close(t *testing.T) {
	backend := NewMemoryBackend()
	err := backend.Close()
	assert.NoError(t, err)
}

func TestMemoryBackend_ThreadSafety(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Test basic thread safety by creating entities concurrently
	// This is a simple test - the edge cases file has more comprehensive concurrency tests
	done := make(chan bool, 2)

	// Goroutine 1: Create entities
	go func() {
		for i := 0; i < 10; i++ {
			entity := mcp.Entity{
				ID:         fmt.Sprintf("thread-1-%d", i),
				Name:       fmt.Sprintf("Thread 1 Entity %d", i),
				EntityType: mcp.EntityTypePerson,
				CreatedAt:  time.Now(),
			}
			backend.CreateEntities(ctx, []mcp.Entity{entity})
		}
		done <- true
	}()

	// Goroutine 2: Search entities
	go func() {
		for i := 0; i < 10; i++ {
			backend.SearchEntities(ctx, "Thread")
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify we can still access the backend
	stats, err := backend.GetStatistics(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats["entities"], 0)
}

// Test the helper function
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		str      string
		substr   string
		expected bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "foo", false},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s contains %s", tt.str, tt.substr), func(t *testing.T) {
			result := containsIgnoreCase(tt.str, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}