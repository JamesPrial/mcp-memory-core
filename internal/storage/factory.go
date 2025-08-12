package storage

import (
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
)

// NewBackend creates a new storage backend based on the configuration
func NewBackend(cfg *config.Settings) (Backend, error) {
	if cfg == nil {
		return nil, errors.New(errors.ErrCodeConfiguration, "Configuration cannot be nil")
	}
	switch cfg.StorageType {
	case "sqlite":
		if cfg.StoragePath == "" {
			return nil, errors.New(errors.ErrCodeConfiguration, "Storage path is required for SQLite backend")
		}
		return NewSqliteBackend(cfg.StoragePath, cfg.Sqlite.WALMode)
	case "memory", "":
		return NewMemoryBackend(), nil
	default:
		return nil, errors.Newf(errors.ErrCodeConfiguration, "Unsupported storage type: %s", cfg.StorageType)
	}
}