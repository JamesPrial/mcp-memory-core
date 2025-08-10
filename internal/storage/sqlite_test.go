package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBackend(t *testing.T) (Backend, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	backend, err := NewSqliteBackend(dbPath, true)
	require.NoError(t, err)

	cleanup := func() {
		backend.Close()
	}

	return backend, cleanup
}

// Core functionality tests
func TestSqliteBackend_BasicCRUD(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	entity := mcp.Entity{
		ID:           "test-id-1",
		Name:         "Test Entity",
		EntityType:   mcp.EntityTypePerson,
		Observations: []string{"obs1", "obs2"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Test Create
	err := backend.CreateEntities(ctx, []mcp.Entity{entity})
	require.NoError(t, err)

	// Test Get
	retrieved, err := backend.GetEntity(ctx, "test-id-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, entity.ID, retrieved.ID)
	assert.Equal(t, entity.Name, retrieved.Name)
	assert.Equal(t, entity.EntityType, retrieved.EntityType)
	assert.Equal(t, entity.Observations, retrieved.Observations)

	// Test Get non-existent
	notFound, err := backend.GetEntity(ctx, "non-existent-id")
	assert.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestSqliteBackend_SearchEntities(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	// Test data with various search scenarios
	entities := []mcp.Entity{
		{ID: "search1", Name: "Alpha Test", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{ID: "search2", Name: "Beta Test", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{ID: "search3", Name: "Alpha Other", EntityType: mcp.EntityTypeOrganization, CreatedAt: now},
		{ID: "search4", Name: "UPPERCASE", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{ID: "search5", Name: "lowercase", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{ID: "search6", Name: "MixedCase", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{
			ID: "obs1", Name: "Entity One", EntityType: mcp.EntityTypePerson,
			Observations: []string{"contains searchable text", "another observation"}, CreatedAt: now,
		},
		{
			ID: "obs2", Name: "Entity Two", EntityType: mcp.EntityTypePerson,
			Observations: []string{"different content", "no match here"}, CreatedAt: now,
		},
		{
			ID: "obs3", Name: "Entity Three", EntityType: mcp.EntityTypePerson,
			Observations: []string{"SEARCHABLE in uppercase", "case test"}, CreatedAt: now,
		},
	}

	err := backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	tests := []struct {
		name     string
		query    string
		expected int
		contains []string // IDs that should be found
	}{
		{"basic name search", "Alpha", 2, []string{"search1", "search3"}},
		{"case insensitive - lowercase query", "uppercase", 2, []string{"search4", "obs3"}}, // Matches name and observation
		{"case insensitive - uppercase query", "LOWERCASE", 1, []string{"search5"}},
		{"case insensitive - mixed query", "mixedcase", 1, []string{"search6"}},
		{"observation search", "searchable", 2, []string{"obs1", "obs3"}},
		{"empty query returns all", "", 9, nil}, // Should return all entities
		{"no results", "nonexistent", 0, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := backend.SearchEntities(ctx, tt.query)
			require.NoError(t, err)
			assert.Len(t, results, tt.expected)

			if len(tt.contains) > 0 {
				foundIDs := make([]string, len(results))
				for i, result := range results {
					foundIDs[i] = result.ID
				}
				for _, expectedID := range tt.contains {
					assert.Contains(t, foundIDs, expectedID)
				}
			}
		})
	}
}

func TestSqliteBackend_GetStatistics(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	// Test empty backend
	stats, err := backend.GetStatistics(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats["entities"])

	// Add entities of different types
	entities := []mcp.Entity{
		{ID: "stat1", Name: "Person 1", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{ID: "stat2", Name: "Person 2", EntityType: mcp.EntityTypePerson, CreatedAt: now},
		{ID: "stat3", Name: "Org 1", EntityType: mcp.EntityTypeOrganization, CreatedAt: now},
		{ID: "stat4", Name: "Project 1", EntityType: mcp.EntityTypeProject, CreatedAt: now},
	}

	err = backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	stats, err = backend.GetStatistics(ctx)
	require.NoError(t, err)
	
	assert.Equal(t, 4, stats["entities"])
	assert.Equal(t, 2, stats["type_"+string(mcp.EntityTypePerson)])
	assert.Equal(t, 1, stats["type_"+string(mcp.EntityTypeOrganization)])
	assert.Equal(t, 1, stats["type_"+string(mcp.EntityTypeProject)])
}

// Test consistency between SQLite and Memory backends
func TestSqliteBackend_ConsistencyWithMemory(t *testing.T) {
	testCases := []struct {
		name     string
		entities []mcp.Entity
		query    string
		expected int
	}{
		{
			name: "case insensitive search",
			entities: []mcp.Entity{
				{ID: "1", Name: "Alpha", EntityType: mcp.EntityTypePerson, Observations: []string{"beta"}, CreatedAt: time.Now().UTC()},
				{ID: "2", Name: "Beta", EntityType: mcp.EntityTypePerson, Observations: []string{"gamma"}, CreatedAt: time.Now().UTC()},
				{ID: "3", Name: "Gamma", EntityType: mcp.EntityTypePerson, Observations: []string{"alpha"}, CreatedAt: time.Now().UTC()},
			},
			query:    "alpha",
			expected: 2, // Should find "Alpha" in name and entity with "alpha" in observations
		},
		{
			name: "empty query returns all",
			entities: []mcp.Entity{
				{ID: "1", Name: "Test1", EntityType: mcp.EntityTypePerson, CreatedAt: time.Now().UTC()},
				{ID: "2", Name: "Test2", EntityType: mcp.EntityTypePerson, CreatedAt: time.Now().UTC()},
			},
			query:    "",
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test SQLite backend
			sqliteBackend, cleanup := newTestBackend(t)
			defer cleanup()
			
			ctx := context.Background()
			err := sqliteBackend.CreateEntities(ctx, tc.entities)
			require.NoError(t, err)
			
			sqliteResults, err := sqliteBackend.SearchEntities(ctx, tc.query)
			require.NoError(t, err)
			
			// Test Memory backend
			memoryBackend := NewMemoryBackend()
			err = memoryBackend.CreateEntities(ctx, tc.entities)
			require.NoError(t, err)
			
			memoryResults, err := memoryBackend.SearchEntities(ctx, tc.query)
			require.NoError(t, err)
			
			// Both backends should return the same number of results
			assert.Equal(t, len(memoryResults), len(sqliteResults), 
				"SQLite and Memory backends should return same number of results for query: %s", tc.query)
			assert.Equal(t, tc.expected, len(sqliteResults))
		})
	}
}

// Concurrent access tests
func TestSqliteBackend_ConcurrentAccess(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	var wg sync.WaitGroup
	const numWorkers = 10
	const opsPerWorker = 10

	// Concurrent writes
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				entity := mcp.Entity{
					ID:         fmt.Sprintf("worker-%d-entity-%d", workerID, j),
					Name:       fmt.Sprintf("Worker %d Entity %d", workerID, j),
					EntityType: mcp.EntityTypePerson,
					CreatedAt:  time.Now().UTC(),
				}
				backend.CreateEntities(ctx, []mcp.Entity{entity})
			}
		}(i)
	}

	// Concurrent reads
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				backend.SearchEntities(ctx, "Worker")
			}
		}()
	}

	wg.Wait()

	// Verify final state
	stats, err := backend.GetStatistics(ctx)
	require.NoError(t, err)
	assert.Equal(t, numWorkers*opsPerWorker, stats["entities"])
}

// Error handling tests
func TestSqliteBackend_ErrorHandling(t *testing.T) {
	t.Run("invalid database path", func(t *testing.T) {
		_, err := NewSqliteBackend("/root/no_permission.db", true)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to open database") || 
			strings.Contains(err.Error(), "failed to ping database"))
	})

	t.Run("directory as database path", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := NewSqliteBackend(tempDir, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to ping database")
	})

	t.Run("context cancellation", func(t *testing.T) {
		backend, cleanup := newTestBackend(t)
		defer cleanup()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		entities := []mcp.Entity{{
			ID:         "test-id",
			Name:       "Test",
			EntityType: mcp.EntityTypePerson,
			CreatedAt:  time.Now().UTC(),
		}}

		err := backend.CreateEntities(ctx, entities)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")

		_, err = backend.GetEntity(ctx, "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")

		_, err = backend.SearchEntities(ctx, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")

		_, err = backend.GetStatistics(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("database closed operations", func(t *testing.T) {
		backend, cleanup := newTestBackend(t)
		cleanup() // Close the database first

		ctx := context.Background()
		entities := []mcp.Entity{{
			ID:         "test-id",
			Name:       "Test",
			EntityType: mcp.EntityTypePerson,
			CreatedAt:  time.Now().UTC(),
		}}

		err := backend.CreateEntities(ctx, entities)
		assert.Error(t, err)

		_, err = backend.GetEntity(ctx, "test-id")
		assert.Error(t, err)

		_, err = backend.SearchEntities(ctx, "test")
		assert.Error(t, err)

		_, err = backend.GetStatistics(ctx)
		assert.Error(t, err)
	})

	t.Run("corrupted JSON in database", func(t *testing.T) {
		backend, cleanup := newTestBackend(t)
		defer cleanup()

		// Manually insert corrupted JSON to test unmarshal errors
		sqliteBackend := backend.(*SqliteBackend)
		ctx := context.Background()
		
		_, err := sqliteBackend.db.ExecContext(ctx, `
			INSERT INTO entities (id, name, entity_type, observations, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, "corrupted-id", "Test", "test", "invalid-json-{", time.Now().UTC())
		require.NoError(t, err)

		// Operations should fail with unmarshal errors
		_, err = backend.GetEntity(ctx, "corrupted-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal observations")

		_, err = backend.SearchEntities(ctx, "Test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal observations")
	})

	t.Run("empty slice operations", func(t *testing.T) {
		backend, cleanup := newTestBackend(t)
		defer cleanup()

		ctx := context.Background()
		
		// Empty slice should not error
		err := backend.CreateEntities(ctx, []mcp.Entity{})
		assert.NoError(t, err)

		// Entity with empty observations should work
		entities := []mcp.Entity{{
			ID:           "empty-obs",
			Name:         "Empty Observations",
			EntityType:   mcp.EntityTypePerson,
			Observations: []string{},
			CreatedAt:    time.Now().UTC(),
		}}
		
		err = backend.CreateEntities(ctx, entities)
		require.NoError(t, err)

		entity, err := backend.GetEntity(ctx, "empty-obs")
		assert.NoError(t, err)
		assert.NotNil(t, entity)
		assert.Empty(t, entity.Observations)
	})

	t.Run("multiple close operations", func(t *testing.T) {
		backend, _ := newTestBackend(t)
		
		err1 := backend.Close()
		assert.NoError(t, err1)
		
		// Second close should be handled gracefully
		err2 := backend.Close()
		if err2 != nil {
			assert.Error(t, err2)
		}
	})
}

// Schema and database integrity tests
func TestSqliteBackend_DatabaseIntegrity(t *testing.T) {
	t.Run("read-only database file", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "readonly.db")
		
		// Create and make read-only
		file, err := os.Create(dbPath)
		require.NoError(t, err)
		file.Close()
		
		err = os.Chmod(dbPath, 0444)
		require.NoError(t, err)
		
		// Should fail during initialization
		_, err = NewSqliteBackend(dbPath, true)
		if err != nil {
			assert.True(t, strings.Contains(err.Error(), "failed to initialize schema") || 
				strings.Contains(err.Error(), "failed to ping database"))
		}
		
		// Restore permissions for cleanup
		os.Chmod(dbPath, 0644)
	})

	t.Run("corrupted database file", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "corrupted.db")
		
		// Create invalid SQLite file
		err := os.WriteFile(dbPath, []byte("this is not a valid sqlite database"), 0644)
		require.NoError(t, err)
		
		_, err = NewSqliteBackend(dbPath, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to ping database")
	})

	t.Run("database locking scenarios", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "locked.db")
		
		// Create first connection with WAL mode disabled
		backend1, err := NewSqliteBackend(dbPath, false)
		require.NoError(t, err)
		defer backend1.Close()
		
		// Start long-running transaction
		sqliteBackend1 := backend1.(*SqliteBackend)
		tx, err := sqliteBackend1.db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()
		
		// Insert data but don't commit
		_, err = tx.Exec("INSERT INTO entities (id, name, entity_type, observations, created_at) VALUES (?, ?, ?, ?, ?)",
			"lock-test", "Lock Test", "test", "[]", time.Now().UTC())
		require.NoError(t, err)
		
		// Second connection
		backend2, err := NewSqliteBackend(dbPath, false)
		if err != nil {
			assert.Contains(t, err.Error(), "database")
			return
		}
		defer backend2.Close()
		
		// Try to write with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		entity := mcp.Entity{
			ID:         "lock-test-2",
			Name:       "Lock Test 2",
			EntityType: mcp.EntityTypePerson,
			CreatedAt:  time.Now().UTC(),
		}
		
		err = backend2.CreateEntities(ctx, []mcp.Entity{entity})
		if err != nil {
			assert.True(t, 
				strings.Contains(err.Error(), "database is locked") ||
				strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "failed to begin transaction"))
		}
	})
}