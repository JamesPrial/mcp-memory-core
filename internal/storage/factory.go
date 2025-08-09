package storage

import (
	"fmt"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

// NewBackend creates a new storage backend based on the configuration
func NewBackend(config *config.Settings) (Backend, error) {
	switch config.StorageType {
	case "sqlite":
		// For now, return a memory backend since SQLite isn't implemented yet
		// In a real implementation, this would create an SQLite backend
		return NewMemoryBackend(), nil
	case "mock", "memory", "":
		return NewMemoryBackend(), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.StorageType)
	}
}