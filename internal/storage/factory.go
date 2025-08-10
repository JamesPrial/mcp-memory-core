package storage

import (
	"fmt"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

// NewBackend creates a new storage backend based on the configuration
func NewBackend(cfg *config.Settings) (Backend, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	switch cfg.StorageType {
	case "sqlite":
		if cfg.StoragePath == "" {
			return nil, fmt.Errorf("storage path is required for SQLite backend")
		}
		return NewSqliteBackend(cfg.StoragePath, cfg.Sqlite.WALMode)
	case "memory", "":
		return NewMemoryBackend(), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.StorageType)
	}
}