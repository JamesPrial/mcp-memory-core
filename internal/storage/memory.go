package storage

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
)

// MemoryBackend is a simple in-memory storage backend for testing
type MemoryBackend struct {
	mu       sync.RWMutex
	entities map[string]mcp.Entity
	logger   *slog.Logger
}

// NewMemoryBackend creates a new memory-based storage backend
func NewMemoryBackend() *MemoryBackend {
	logger := logging.GetGlobalLogger("storage.memory")
	
	logger.Info("Creating memory backend")
	
	return &MemoryBackend{
		entities: make(map[string]mcp.Entity),
		logger:   logger,
	}
}

// CreateEntities creates new entities in memory
func (m *MemoryBackend) CreateEntities(ctx context.Context, entities []mcp.Entity) error {
	if len(entities) == 0 {
		m.logger.DebugContext(ctx, "No entities to create")
		return nil
	}

	timer := logging.StartTimer(ctx, m.logger, "createEntities")
	defer timer.End()
	
	m.logger.InfoContext(ctx, "Creating entities in memory",
		slog.Int("count", len(entities)),
	)
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		m.logger.WarnContext(ctx, "Create entities operation canceled")
		return ctx.Err()
	default:
	}
	
	startTime := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Validate all entities first before making any changes
	for _, entity := range entities {
		// Check for empty string IDs
		if strings.TrimSpace(entity.ID) == "" {
			m.logger.ErrorContext(ctx, "Entity ID cannot be empty",
				slog.String("entity_id", entity.ID),
			)
			return errors.New(errors.ErrCodeValidationRequired, "Entity ID cannot be empty or whitespace-only")
		}
		// Check for duplicate IDs
		if _, exists := m.entities[entity.ID]; exists {
			m.logger.WarnContext(ctx, "Entity already exists",
				slog.String("entity_id", entity.ID),
				slog.String("entity_name", entity.Name),
			)
			return errors.Newf(errors.ErrCodeEntityAlreadyExists, "Entity with ID '%s' already exists", entity.ID)
		}
	}
	
	// All validations passed, now create the entities
	for _, entity := range entities {
		m.entities[entity.ID] = entity
		m.logger.DebugContext(ctx, "Entity stored in memory",
			slog.String("entity_id", entity.ID),
			slog.String("entity_name", entity.Name),
			slog.Int("observations_count", len(entity.Observations)),
		)
	}
	
	duration := time.Since(startTime)
	m.logger.InfoContext(ctx, "Successfully created entities in memory",
		slog.Int("count", len(entities)),
		slog.Duration("duration", duration),
		slog.Int("total_entities", len(m.entities)),
	)
	
	return nil
}

// GetEntity retrieves an entity by ID from memory
func (m *MemoryBackend) GetEntity(ctx context.Context, id string) (*mcp.Entity, error) {
	timer := logging.StartTimer(ctx, m.logger, "getEntity")
	defer timer.End()
	
	m.logger.DebugContext(ctx, "Getting entity from memory",
		slog.String("entity_id", id),
	)
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		m.logger.WarnContext(ctx, "Get entity operation canceled")
		return nil, ctx.Err()
	default:
	}
	
	startTime := time.Now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entity, exists := m.entities[id]
	duration := time.Since(startTime)
	
	if !exists {
		m.logger.DebugContext(ctx, "Entity not found in memory",
			slog.String("entity_id", id),
			slog.Duration("duration", duration),
		)
		return nil, nil // Return nil for consistency with SQLite backend (not found returns nil, not error)
	}
	
	m.logger.DebugContext(ctx, "Entity retrieved from memory",
		slog.String("entity_id", id),
		slog.String("entity_name", entity.Name),
		slog.Duration("duration", duration),
		slog.Int("observations_count", len(entity.Observations)),
	)
	
	return &entity, nil
}

// SearchEntities searches for entities by query in memory
func (m *MemoryBackend) SearchEntities(ctx context.Context, query string) ([]mcp.Entity, error) {
	timer := logging.StartTimer(ctx, m.logger, "searchEntities")
	defer timer.End()
	
	m.logger.InfoContext(ctx, "Searching entities in memory",
		slog.String("query", query),
	)
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		m.logger.WarnContext(ctx, "Search entities operation canceled")
		return nil, ctx.Err()
	default:
	}
	
	startTime := time.Now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var results []mcp.Entity
	entitiesScanned := 0
	
	for _, entity := range m.entities {
		entitiesScanned++
		// Simple string matching in name and observations
		if containsIgnoreCase(entity.Name, query) {
			results = append(results, entity)
			m.logger.DebugContext(ctx, "Entity matched by name",
				slog.String("entity_id", entity.ID),
				slog.String("entity_name", entity.Name),
				slog.String("query", query),
			)
			continue
		}
		
		for _, obs := range entity.Observations {
			if containsIgnoreCase(obs, query) {
				results = append(results, entity)
				m.logger.DebugContext(ctx, "Entity matched by observation",
					slog.String("entity_id", entity.ID),
					slog.String("entity_name", entity.Name),
					slog.String("query", query),
					slog.String("matching_observation", obs),
				)
				break
			}
		}
	}
	
	duration := time.Since(startTime)
	m.logger.InfoContext(ctx, "Search completed",
		slog.String("query", query),
		slog.Int("entities_scanned", entitiesScanned),
		slog.Int("results_count", len(results)),
		slog.Duration("duration", duration),
	)
	
	return results, nil
}

// GetStatistics returns statistics about the memory storage
func (m *MemoryBackend) GetStatistics(ctx context.Context) (map[string]int, error) {
	timer := logging.StartTimer(ctx, m.logger, "getStatistics")
	defer timer.End()
	
	m.logger.DebugContext(ctx, "Getting statistics from memory")
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		m.logger.WarnContext(ctx, "Get statistics operation canceled")
		return nil, ctx.Err()
	default:
	}
	
	startTime := time.Now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := map[string]int{
		"entities": len(m.entities),
	}
	
	duration := time.Since(startTime)
	m.logger.InfoContext(ctx, "Statistics retrieved from memory",
		slog.Int("entities_count", stats["entities"]),
		slog.Duration("duration", duration),
	)
	
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