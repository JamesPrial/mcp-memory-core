package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/mitchellh/mapstructure"
)

// MCPTool represents an MCP tool with its metadata
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Manager handles knowledge management operations for MCP tools
type Manager struct {
	storage storage.Backend
}

// NewManager creates a new Manager instance with the provided storage backend
func NewManager(storage storage.Backend) *Manager {
	return &Manager{
		storage: storage,
	}
}

// HandleListTools returns the list of available MCP tools
func (m *Manager) HandleListTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "memory__create_entities",
			Description: "Create new entities in the knowledge base",
		},
		{
			Name:        "memory__search",
			Description: "Search for entities in the knowledge base",
		},
		{
			Name:        "memory__get_entity",
			Description: "Get a specific entity by ID from the knowledge base",
		},
		{
			Name:        "memory__get_statistics",
			Description: "Get statistics about the knowledge base",
		},
	}
}

// HandleCallTool handles tool calls for various MCP operations
func (m *Manager) HandleCallTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "memory__create_entities":
		return m.handleCreateEntities(ctx, args)
	case "memory__search":
		return m.handleSearch(ctx, args)
	case "memory__get_entity":
		return m.handleGetEntity(ctx, args)
	case "memory__get_statistics":
		return m.handleGetStatistics(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// handleCreateEntities processes entity creation requests
func (m *Manager) handleCreateEntities(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	entitiesRaw, ok := args["entities"]
	if !ok {
		return nil, fmt.Errorf("entities parameter is required")
	}

	entitiesSlice, ok := entitiesRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("entities must be an array")
	}

	var entities []mcp.Entity
	for i, entityRaw := range entitiesSlice {
		var entity mcp.Entity
		
		// Use mapstructure to convert map[string]interface{} to mcp.Entity
		if err := mapstructure.Decode(entityRaw, &entity); err != nil {
			return nil, fmt.Errorf("failed to decode entity at index %d: %w", i, err)
		}

		// Generate ID if not provided
		if entity.ID == "" {
			entity.ID = generateEntityID(entity.Name)
		}

		// Set created timestamp
		if entity.CreatedAt.IsZero() {
			entity.CreatedAt = time.Now()
		}

		entities = append(entities, entity)
	}

	if err := m.storage.CreateEntities(ctx, entities); err != nil {
		return nil, fmt.Errorf("failed to create entities: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"count":   len(entities),
	}, nil
}

// handleSearch processes search requests
func (m *Manager) handleSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"]
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	queryStr, ok := query.(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	entities, err := m.storage.SearchEntities(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities: %w", err)
	}

	return map[string]interface{}{
		"entities": entities,
		"count":    len(entities),
	}, nil
}

// handleGetEntity processes get entity requests
func (m *Manager) handleGetEntity(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"]
	if !ok {
		return nil, fmt.Errorf("id parameter is required")
	}

	idStr, ok := id.(string)
	if !ok {
		return nil, fmt.Errorf("id must be a string")
	}

	entity, err := m.storage.GetEntity(ctx, idStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	return entity, nil
}

// handleGetStatistics processes get statistics requests
func (m *Manager) handleGetStatistics(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	stats, err := m.storage.GetStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	return stats, nil
}

// generateEntityID generates a simple ID based on entity name
func generateEntityID(name string) string {
	return fmt.Sprintf("entity_%d", time.Now().UnixNano())
}