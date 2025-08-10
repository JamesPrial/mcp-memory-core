package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	_ "github.com/mattn/go-sqlite3"
)

type SqliteBackend struct {
	db *sql.DB
}

// NewSqliteBackend creates a new SQLite backend with the specified database path and WAL mode setting
func NewSqliteBackend(dbPath string, walMode bool) (Backend, error) {
	// Configure connection string with appropriate settings
	connStr := dbPath
	if walMode {
		connStr += "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=true"
	} else {
		connStr += "?_synchronous=FULL&_cache_size=1000&_foreign_keys=true"
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	backend := &SqliteBackend{db: db}

	// Initialize the database schema
	if err := backend.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return backend, nil
}

// initSchema creates the necessary tables for the database
func (s *SqliteBackend) initSchema() error {
	createEntitiesTable := `
	CREATE TABLE IF NOT EXISTS entities (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		observations TEXT, -- JSON array stored as text
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);
	CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(entity_type);
	CREATE INDEX IF NOT EXISTS idx_entities_created_at ON entities(created_at);
	CREATE INDEX IF NOT EXISTS idx_entities_updated_at ON entities(updated_at);
	`

	_, err := s.db.Exec(createEntitiesTable)
	if err != nil {
		return err
	}

	// Migration: Add updated_at column if it doesn't exist (for existing databases)
	_, err = s.db.Exec(`
		ALTER TABLE entities ADD COLUMN updated_at DATETIME;
	`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		// Only return error if it's not a "column already exists" error
		return fmt.Errorf("failed to add updated_at column: %w", err)
	}

	// Update any existing records that don't have updated_at
	_, err = s.db.Exec(`
		UPDATE entities SET updated_at = created_at WHERE updated_at IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("failed to initialize updated_at values: %w", err)
	}

	return nil
}

// CreateEntities inserts multiple entities into the database
func (s *SqliteBackend) CreateEntities(ctx context.Context, entities []mcp.Entity) error {
	if len(entities) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO entities (id, name, entity_type, observations, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			entity_type = excluded.entity_type,
			observations = excluded.observations,
			updated_at = excluded.updated_at
			-- created_at is NOT updated, preserving original creation time
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, entity := range entities {
		// Check for empty string IDs
		if strings.TrimSpace(entity.ID) == "" {
			return fmt.Errorf("entity ID cannot be empty or whitespace-only")
		}
		
		// Serialize observations to JSON
		observationsJSON, err := json.Marshal(entity.Observations)
		if err != nil {
			return fmt.Errorf("failed to marshal observations for entity %s: %w", entity.ID, err)
		}

		_, err = stmt.ExecContext(ctx, 
			entity.ID, 
			entity.Name, 
			entity.EntityType, 
			string(observationsJSON), 
			entity.CreatedAt,
			entity.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert entity %s: %w", entity.ID, err)
		}
	}

	return tx.Commit()
}

// GetEntity retrieves a single entity by ID
func (s *SqliteBackend) GetEntity(ctx context.Context, id string) (*mcp.Entity, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, entity_type, observations, created_at, updated_at
		FROM entities
		WHERE id = ?
	`, id)

	var entity mcp.Entity
	var observationsJSON string

	err := row.Scan(
		&entity.ID,
		&entity.Name,
		&entity.EntityType,
		&observationsJSON,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Entity not found
		}
		return nil, fmt.Errorf("failed to scan entity: %w", err)
	}

	// Deserialize observations from JSON
	if observationsJSON != "" {
		if err := json.Unmarshal([]byte(observationsJSON), &entity.Observations); err != nil {
			return nil, fmt.Errorf("failed to unmarshal observations: %w", err)
		}
	}

	return &entity, nil
}

// SearchEntities searches for entities by name and observations using case-insensitive LIKE queries
// This implementation matches the memory backend behavior:
// - Case-insensitive search
// - Searches both entity names and observations
// - Empty query returns all entities
func (s *SqliteBackend) SearchEntities(ctx context.Context, query string) ([]mcp.Entity, error) {
	var rows *sql.Rows
	var err error
	
	// Handle empty query - return all entities (matches memory backend behavior)
	if query == "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, name, entity_type, observations, created_at, updated_at
			FROM entities
			ORDER BY created_at DESC
		`)
	} else {
		// Search both name and observations with case-insensitive matching
		searchPattern := "%" + query + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, name, entity_type, observations, created_at, updated_at
			FROM entities
			WHERE name LIKE ? COLLATE NOCASE 
			   OR observations LIKE ? COLLATE NOCASE
			ORDER BY created_at DESC
		`, searchPattern, searchPattern)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer rows.Close()

	var entities []mcp.Entity
	for rows.Next() {
		var entity mcp.Entity
		var observationsJSON string

		err := rows.Scan(
			&entity.ID,
			&entity.Name,
			&entity.EntityType,
			&observationsJSON,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity: %w", err)
		}

		// Deserialize observations from JSON
		if observationsJSON != "" {
			if err := json.Unmarshal([]byte(observationsJSON), &entity.Observations); err != nil {
				return nil, fmt.Errorf("failed to unmarshal observations: %w", err)
			}
		}

		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return entities, nil
}

// GetStatistics returns statistics about the entities in the database
func (s *SqliteBackend) GetStatistics(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	// Get total entity count
	var totalCount int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entities").Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get total entity count: %w", err)
	}
	stats["entities"] = totalCount

	// Get entity counts by type
	rows, err := s.db.QueryContext(ctx, `
		SELECT entity_type, COUNT(*) 
		FROM entities 
		GROUP BY entity_type
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query entity counts by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entityType string
		var count int
		if err := rows.Scan(&entityType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan entity type count: %w", err)
		}
		// Use "type_" prefix to match memory backend convention
		if entityType != "" {
			stats[fmt.Sprintf("type_%s", entityType)] = count
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over entity type rows: %w", err)
	}

	return stats, nil
}

// Close closes the database connection
func (s *SqliteBackend) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}