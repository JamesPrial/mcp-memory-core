package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
)

// MemoryBackend is a simple in-memory storage backend for testing
type MemoryBackend struct {
	mu       sync.RWMutex
	entities map[string]mcp.Entity
}

// NewMemoryBackend creates a new memory-based storage backend
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		entities: make(map[string]mcp.Entity),
	}
}

// CreateEntities creates new entities in memory
func (m *MemoryBackend) CreateEntities(ctx context.Context, entities []mcp.Entity) error {
	// Check context cancellation early
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Validate all entities first before making any changes
	for _, entity := range entities {
		// Check for empty string IDs
		if strings.TrimSpace(entity.ID) == "" {
			return fmt.Errorf("entity ID cannot be empty or whitespace-only")
		}
		// Check for duplicate IDs
		if _, exists := m.entities[entity.ID]; exists {
			return fmt.Errorf("entity with ID '%s' already exists", entity.ID)
		}
	}
	
	// All validations passed, now create the entities
	for _, entity := range entities {
		m.entities[entity.ID] = entity
	}
	return nil
}

// GetEntity retrieves an entity by ID from memory
func (m *MemoryBackend) GetEntity(ctx context.Context, id string) (*mcp.Entity, error) {
	// Check context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entity, exists := m.entities[id]
	if !exists {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	return &entity, nil
}

// SearchEntities searches for entities by query in memory
func (m *MemoryBackend) SearchEntities(ctx context.Context, query string) ([]mcp.Entity, error) {
	// Check context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var results []mcp.Entity
	for _, entity := range m.entities {
		// Simple string matching in name and observations
		if containsIgnoreCase(entity.Name, query) {
			results = append(results, entity)
			continue
		}
		
		for _, obs := range entity.Observations {
			if containsIgnoreCase(obs, query) {
				results = append(results, entity)
				break
			}
		}
	}
	return results, nil
}

// GetStatistics returns statistics about the memory storage
func (m *MemoryBackend) GetStatistics(ctx context.Context) (map[string]int, error) {
	// Check context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := map[string]int{
		"entities": len(m.entities),
	}
	
	// Count entities by type
	typeCounts := make(map[string]int)
	for _, entity := range m.entities {
		if entity.EntityType != "" {
			typeCounts[string(entity.EntityType)]++
		}
	}
	
	// Add type counts to stats
	for entityType, count := range typeCounts {
		stats["type_"+entityType] = count
	}
	
	return stats, nil
}

// Close closes the memory backend (no-op)
func (m *MemoryBackend) Close() error {
	return nil
}

// containsIgnoreCase performs case-insensitive substring matching
func containsIgnoreCase(str, substr string) bool {
	str = strings.ToLower(str)
	substr = strings.ToLower(substr)
	return strings.Contains(str, substr)
}