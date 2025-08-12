package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	_ "github.com/mattn/go-sqlite3"
)

type SqliteBackend struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSqliteBackend creates a new SQLite backend with the specified database path and WAL mode setting
func NewSqliteBackend(dbPath string, walMode bool) (Backend, error) {
	logger := logging.GetGlobalLogger("storage.sqlite")
	
	logger.Info("Creating SQLite backend",
		slog.String("db_path", dbPath),
		slog.Bool("wal_mode", walMode),
	)
	
	// Configure connection string with appropriate settings
	connStr := dbPath
	if walMode {
		connStr += "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=true"
	} else {
		connStr += "?_synchronous=FULL&_cache_size=1000&_foreign_keys=true"
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		logger.Error("Failed to open database connection",
			slog.String("connection_string", connStr),
			slog.String("error", err.Error()),
		)
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to open database connection")
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		logger.Error("Database connection test failed",
			slog.String("error", err.Error()),
		)
		db.Close()
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Database connection test failed")
	}

	backend := &SqliteBackend{
		db:     db,
		logger: logger,
	}

	// Initialize the database schema
	if err := backend.initSchema(); err != nil {
		db.Close()
		return nil, errors.Wrap(err, errors.ErrCodeStorageInitialization, "Failed to initialize database schema")
	}

	logger.Info("SQLite backend initialized successfully")
	return backend, nil
}

// initSchema creates the necessary tables for the database
func (s *SqliteBackend) initSchema() error {
	s.logger.Debug("Initializing database schema")
	
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

	startTime := time.Now()
	_, err := s.db.Exec(createEntitiesTable)
	duration := time.Since(startTime)
	
	if err != nil {
		s.logger.Error("Failed to create entities table",
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		return errors.Wrap(err, errors.ErrCodeStorageInitialization, "Failed to create entities table")
	}
	
	s.logger.Debug("Entities table created/verified",
		slog.Duration("duration", duration),
	)

	// Migration: Add updated_at column if it doesn't exist (for existing databases)
	_, err = s.db.Exec(`
		ALTER TABLE entities ADD COLUMN updated_at DATETIME;
	`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		// Only return error if it's not a "column already exists" error
		return errors.Wrap(err, errors.ErrCodeStorageInitialization, "Failed to add updated_at column")
	}

	// Update any existing records that don't have updated_at
	_, err = s.db.Exec(`
		UPDATE entities SET updated_at = created_at WHERE updated_at IS NULL;
	`)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeStorageInitialization, "Failed to initialize updated_at values")
	}

	return nil
}

// CreateEntities inserts multiple entities into the database
func (s *SqliteBackend) CreateEntities(ctx context.Context, entities []mcp.Entity) error {
	if len(entities) == 0 {
		s.logger.DebugContext(ctx, "No entities to create")
		return nil
	}

	timer := logging.StartTimer(ctx, s.logger, "createEntities")
	defer timer.End()
	
	s.logger.InfoContext(ctx, "Creating entities in database",
		slog.Int("count", len(entities)),
	)

	startTime := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to begin transaction",
			slog.String("error", err.Error()),
		)
		return errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to begin transaction")
	}
	
	s.logger.DebugContext(ctx, "Database transaction started",
		slog.Duration("tx_start_duration", time.Since(startTime)),
	)
	defer tx.Rollback()

	prepareTime := time.Now()
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
		s.logger.ErrorContext(ctx, "Failed to prepare insert statement",
			slog.String("error", err.Error()),
		)
		return errors.Wrap(err, errors.ErrCodeStorageInvalidQuery, "Failed to prepare insert statement")
	}
	defer stmt.Close()
	
	s.logger.DebugContext(ctx, "Statement prepared",
		slog.Duration("prepare_duration", time.Since(prepareTime)),
	)

	for _, entity := range entities {
		// Check for empty string IDs
		if strings.TrimSpace(entity.ID) == "" {
			return errors.New(errors.ErrCodeValidationRequired, "Entity ID cannot be empty or whitespace-only")
		}
		
		// Serialize observations to JSON
		observationsJSON, err := json.Marshal(entity.Observations)
		if err != nil {
			return errors.Wrapf(err, errors.ErrCodeStorageInvalidQuery, "Failed to marshal observations for entity %s", entity.ID)
		}

		insertTime := time.Now()
		_, err = stmt.ExecContext(ctx, 
			entity.ID, 
			entity.Name, 
			entity.EntityType, 
			string(observationsJSON), 
			entity.CreatedAt,
			entity.UpdatedAt,
		)
		insertDuration := time.Since(insertTime)
		
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to insert entity",
				slog.String("entity_id", entity.ID),
				slog.String("entity_name", entity.Name),
				slog.Duration("insert_duration", insertDuration),
				slog.String("error", err.Error()),
			)
			
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return errors.Wrapf(err, errors.ErrCodeStorageConflict, "Entity with ID %s already exists", entity.ID)
			}
			return errors.Wrapf(err, errors.ErrCodeStorageInvalidQuery, "Failed to insert entity %s", entity.ID)
		}
		
		s.logger.DebugContext(ctx, "Entity inserted",
			slog.String("entity_id", entity.ID),
			slog.String("entity_name", entity.Name),
			slog.Duration("insert_duration", insertDuration),
		)
	}

	// Commit transaction
	commitTime := time.Now()
	if err := tx.Commit(); err != nil {
		s.logger.ErrorContext(ctx, "Failed to commit transaction",
			slog.Duration("commit_duration", time.Since(commitTime)),
			slog.String("error", err.Error()),
		)
		return errors.Wrap(err, errors.ErrCodeStorageTransaction, "Failed to commit database transaction")
	}

	totalDuration := time.Since(startTime)
	s.logger.InfoContext(ctx, "Successfully created entities",
		slog.Int("count", len(entities)),
		slog.Duration("total_duration", totalDuration),
		slog.Duration("commit_duration", time.Since(commitTime)),
	)

	return nil
}

// GetEntity retrieves a single entity by ID
func (s *SqliteBackend) GetEntity(ctx context.Context, id string) (*mcp.Entity, error) {
	timer := logging.StartTimer(ctx, s.logger, "getEntity")
	defer timer.End()
	
	s.logger.DebugContext(ctx, "Getting entity by ID",
		slog.String("entity_id", id),
	)
	
	queryTime := time.Now()
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
	scanDuration := time.Since(queryTime)
	
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.DebugContext(ctx, "Entity not found",
				slog.String("entity_id", id),
				slog.Duration("query_duration", scanDuration),
			)
			return nil, nil // Entity not found
		}
		s.logger.ErrorContext(ctx, "Failed to scan entity from database",
			slog.String("entity_id", id),
			slog.Duration("query_duration", scanDuration),
			slog.String("error", err.Error()),
		)
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to scan entity from database")
	}

	// Deserialize observations from JSON
	if observationsJSON != "" {
		if err := json.Unmarshal([]byte(observationsJSON), &entity.Observations); err != nil {
			s.logger.ErrorContext(ctx, "Failed to unmarshal observations",
				slog.String("entity_id", id),
				slog.String("observations_json", observationsJSON),
				slog.String("error", err.Error()),
			)
			return nil, errors.Wrap(err, errors.ErrCodeStorageInvalidQuery, "Failed to unmarshal observations")
		}
	}

	s.logger.DebugContext(ctx, "Entity retrieved successfully",
		slog.String("entity_id", id),
		slog.String("entity_name", entity.Name),
		slog.Duration("query_duration", scanDuration),
		slog.Int("observations_count", len(entity.Observations)),
	)

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
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to query entities")
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
			return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to scan entity from search results")
		}

		// Deserialize observations from JSON
		if observationsJSON != "" {
			if err := json.Unmarshal([]byte(observationsJSON), &entity.Observations); err != nil {
				return nil, errors.Wrap(err, errors.ErrCodeStorageInvalidQuery, "Failed to unmarshal observations from search results")
			}
		}

		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Error iterating over search results")
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
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to get total entity count")
	}
	stats["entities"] = totalCount

	// Get entity counts by type
	rows, err := s.db.QueryContext(ctx, `
		SELECT entity_type, COUNT(*) 
		FROM entities 
		GROUP BY entity_type
	`)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to query entity counts by type")
	}
	defer rows.Close()

	for rows.Next() {
		var entityType string
		var count int
		if err := rows.Scan(&entityType, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Failed to scan entity type count")
		}
		// Use "type_" prefix to match memory backend convention
		if entityType != "" {
			stats[fmt.Sprintf("type_%s", entityType)] = count
		}
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeStorageConnection, "Error iterating over entity type statistics")
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