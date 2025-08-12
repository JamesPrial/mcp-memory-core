package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
)

// MCPTool represents an MCP tool with its metadata
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Manager handles knowledge management operations for MCP tools
type Manager struct {
	storage     storage.Backend
	logger      *slog.Logger
	auditLogger *logging.AuditLogger
}

// NewManager creates a new Manager instance with the provided storage backend
func NewManager(storage storage.Backend) *Manager {
	logger := logging.GetGlobalLogger("knowledge")
	auditLogger := logging.GetGlobalAuditLogger()
	
	return &Manager{
		storage:     storage,
		logger:      logger,
		auditLogger: auditLogger,
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
	timer := logging.StartTimer(ctx, m.logger, "HandleCallTool")
	
	m.logger.InfoContext(ctx, "Tool call started",
		slog.String("tool_name", toolName),
		slog.Any("args", args),
	)
	
	var result interface{}
	var err error
	
	switch toolName {
	case "memory__create_entities":
		result, err = m.handleCreateEntities(ctx, args)
	case "memory__search":
		result, err = m.handleSearch(ctx, args)
	case "memory__get_entity":
		result, err = m.handleGetEntity(ctx, args)
	case "memory__get_statistics":
		result, err = m.handleGetStatistics(ctx, args)
	default:
		err = errors.Newf(errors.ErrCodeTransportMethodNotFound, "unknown tool: %s", toolName)
	}
	
	duration := timer.EndWithError(err)
	
	if err != nil {
		m.logger.ErrorContext(ctx, "Tool call failed",
			slog.String("tool_name", toolName),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
	} else {
		m.logger.InfoContext(ctx, "Tool call completed successfully",
			slog.String("tool_name", toolName),
			slog.Duration("duration", duration),
		)
	}
	
	return result, err
}

// handleCreateEntities processes entity creation requests
func (m *Manager) handleCreateEntities(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	timer := logging.StartTimer(ctx, m.logger, "createEntities")
	defer timer.End()
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			m.logger.WarnContext(ctx, "Create entities operation timed out")
			return nil, errors.New(errors.ErrCodeContextTimeout, "Operation timed out")
		}
		m.logger.WarnContext(ctx, "Create entities operation canceled")
		return nil, errors.New(errors.ErrCodeContextCanceled, "Operation canceled")
	default:
	}
	
	entitiesRaw, ok := args["entities"]
	if !ok {
		m.logger.WarnContext(ctx, "Create entities missing required field",
			slog.String("field", "entities"),
		)
		return nil, errors.ValidationRequired("entities")
	}

	entitiesSlice, ok := entitiesRaw.([]interface{})
	if !ok {
		m.logger.WarnContext(ctx, "Create entities invalid field type",
			slog.String("field", "entities"),
			slog.String("expected", "array"),
			slog.String("actual", fmt.Sprintf("%T", entitiesRaw)),
		)
		return nil, errors.ValidationInvalid("entities", "must be an array")
	}

	m.logger.InfoContext(ctx, "Creating entities",
		slog.Int("count", len(entitiesSlice)),
	)

	var entities []mcp.Entity
	now := time.Now()
	
	for i, entityRaw := range entitiesSlice {
		// Check context during iteration for large batches
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, errors.New(errors.ErrCodeContextTimeout, "Operation timed out")
			}
			return nil, errors.New(errors.ErrCodeContextCanceled, "Operation canceled")
		default:
		}
		var entity mcp.Entity
		
		// Use mapstructure to convert map[string]interface{} to mcp.Entity
		if err := mapstructure.Decode(entityRaw, &entity); err != nil {
			return nil, errors.Wrapf(err, errors.ErrCodeValidationInvalid, "failed to decode entity at index %d", i)
		}

		// Generate ID if not provided using UUID for global uniqueness
		if entity.ID == "" {
			entity.ID = generateEntityID()
		}

		// Set created timestamp if not provided
		if entity.CreatedAt.IsZero() {
			entity.CreatedAt = now
		}

		// Always set updated timestamp to current time
		entity.UpdatedAt = now

		entities = append(entities, entity)
	}

	// Log before storage operation
	m.logger.InfoContext(ctx, "Storing entities",
		slog.Int("count", len(entities)),
	)
	
	// Audit log for entity creation
	if m.auditLogger != nil {
		for _, entity := range entities {
			details := map[string]interface{}{
				"entity_name":        entity.Name,
				"entity_type":        entity.EntityType,
				"observations_count": len(entity.Observations),
			}
			m.auditLogger.LogEntityCreated(ctx, string(entity.EntityType), entity.ID, details)
		}
	}

	if err := m.storage.CreateEntities(ctx, entities); err != nil {
		m.logger.ErrorContext(ctx, "Failed to store entities",
			slog.Int("count", len(entities)),
			slog.String("error", err.Error()),
		)
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to create entities")
	}

	m.logger.InfoContext(ctx, "Successfully created entities",
		slog.Int("count", len(entities)),
	)

	response := map[string]interface{}{
		"success": true,
		"count":   len(entities),
	}
	
	// Validate JSON marshaling capability
	if _, err := json.Marshal(response); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeTransportMarshal, "Failed to encode response")
	}
	
	return response, nil
}

// handleSearch processes search requests
func (m *Manager) handleSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	timer := logging.StartTimer(ctx, m.logger, "searchEntities")
	defer timer.End()
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			m.logger.WarnContext(ctx, "Search entities operation timed out")
			return nil, errors.New(errors.ErrCodeContextTimeout, "Operation timed out")
		}
		m.logger.WarnContext(ctx, "Search entities operation canceled")
		return nil, errors.New(errors.ErrCodeContextCanceled, "Operation canceled")
	default:
	}
	
	query, ok := args["query"]
	if !ok {
		m.logger.WarnContext(ctx, "Search missing required field",
			slog.String("field", "query"),
		)
		return nil, errors.ValidationRequired("query")
	}

	queryStr, ok := query.(string)
	if !ok {
		m.logger.WarnContext(ctx, "Search invalid field type",
			slog.String("field", "query"),
			slog.String("expected", "string"),
			slog.String("actual", fmt.Sprintf("%T", query)),
		)
		return nil, errors.ValidationInvalid("query", "must be a string")
	}

	m.logger.InfoContext(ctx, "Searching entities",
		slog.String("query", queryStr),
	)

	entities, err := m.storage.SearchEntities(ctx, queryStr)
	if err != nil {
		m.logger.ErrorContext(ctx, "Failed to search entities",
			slog.String("query", queryStr),
			slog.String("error", err.Error()),
		)
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to search entities")
	}

	m.logger.InfoContext(ctx, "Search completed",
		slog.String("query", queryStr),
		slog.Int("results", len(entities)),
	)

	response := map[string]interface{}{
		"entities": entities,
		"count":    len(entities),
	}
	
	// Validate JSON marshaling capability
	if _, err := json.Marshal(response); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeTransportMarshal, "Failed to encode response")
	}
	
	return response, nil
}

// handleGetEntity processes get entity requests
func (m *Manager) handleGetEntity(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	timer := logging.StartTimer(ctx, m.logger, "getEntity")
	defer timer.End()
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			m.logger.WarnContext(ctx, "Get entity operation timed out")
			return nil, errors.New(errors.ErrCodeContextTimeout, "Operation timed out")
		}
		m.logger.WarnContext(ctx, "Get entity operation canceled")
		return nil, errors.New(errors.ErrCodeContextCanceled, "Operation canceled")
	default:
	}
	
	id, ok := args["id"]
	if !ok {
		m.logger.WarnContext(ctx, "Get entity missing required field",
			slog.String("field", "id"),
		)
		return nil, errors.ValidationRequired("id")
	}

	idStr, ok := id.(string)
	if !ok {
		m.logger.WarnContext(ctx, "Get entity invalid field type",
			slog.String("field", "id"),
			slog.String("expected", "string"),
			slog.String("actual", fmt.Sprintf("%T", id)),
		)
		return nil, errors.ValidationInvalid("id", "must be a string")
	}

	m.logger.InfoContext(ctx, "Getting entity",
		slog.String("entity_id", idStr),
	)

	entity, err := m.storage.GetEntity(ctx, idStr)
	if err != nil {
		m.logger.ErrorContext(ctx, "Failed to get entity",
			slog.String("entity_id", idStr),
			slog.String("error", err.Error()),
		)
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to get entity")
	}

	if entity == nil {
		m.logger.InfoContext(ctx, "Entity not found",
			slog.String("entity_id", idStr),
		)
	} else {
		m.logger.InfoContext(ctx, "Entity retrieved successfully",
			slog.String("entity_id", idStr),
			slog.String("entity_name", entity.Name),
		)
	}
	
	// Validate JSON marshaling capability
	if _, err := json.Marshal(entity); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeTransportMarshal, "Failed to encode response")
	}

	return entity, nil
}

// handleGetStatistics processes get statistics requests
func (m *Manager) handleGetStatistics(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	timer := logging.StartTimer(ctx, m.logger, "getStatistics")
	defer timer.End()
	
	// Check context cancellation early
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			m.logger.WarnContext(ctx, "Get statistics operation timed out")
			return nil, errors.New(errors.ErrCodeContextTimeout, "Operation timed out")
		}
		m.logger.WarnContext(ctx, "Get statistics operation canceled")
		return nil, errors.New(errors.ErrCodeContextCanceled, "Operation canceled")
	default:
	}
	
	m.logger.InfoContext(ctx, "Getting statistics")
	
	stats, err := m.storage.GetStatistics(ctx)
	if err != nil {
		m.logger.ErrorContext(ctx, "Failed to get statistics",
			slog.String("error", err.Error()),
		)
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to get statistics")
	}

	m.logger.InfoContext(ctx, "Statistics retrieved successfully")
	
	// Validate JSON marshaling capability
	if _, err := json.Marshal(stats); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeTransportMarshal, "Failed to encode response")
	}

	return stats, nil
}

// generateEntityID generates a globally unique ID using UUID
func generateEntityID() string {
	return uuid.New().String()
}