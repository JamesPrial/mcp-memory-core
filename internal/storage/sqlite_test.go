package storage

import (
	"context"
	"fmt"
	"path/filepath"
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

func TestSqliteBackend_CreateAndGetEntity(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	entities := []mcp.Entity{{
		ID:         "test-id-1",
		Name:       "Test Entity",
		EntityType: "test",
		Observations: []string{"obs1", "obs2"},
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}}

	err := backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	retrieved, err := backend.GetEntity(ctx, "test-id-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, entities[0].ID, retrieved.ID)
}

func TestSqliteBackend_SearchEntities(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	entities := []mcp.Entity{
		{ID: "s1", Name: "Alpha Test"},
		{ID: "s2", Name: "Beta Test"},
		{ID: "s3", Name: "Alpha Other"},
	}
	err := backend.CreateEntities(ctx, entities)
	require.NoError(t, err)

	results, err := backend.SearchEntities(ctx, "Alpha")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSqliteBackend_ConcurrentAccess(t *testing.T) {
	backend, cleanup := newTestBackend(t)
	defer cleanup()

	ctx := context.Background()
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			entity := mcp.Entity{ID: fmt.Sprintf("c%d", i)}
			backend.CreateEntities(ctx, []mcp.Entity{entity})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			backend.SearchEntities(ctx, "c")
		}
		done <- true
	}()

	<-done
	<-done
}