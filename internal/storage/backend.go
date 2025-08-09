// In file: internal/storage/backend.go
package storage

import (
	"context"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
)

type Backend interface {
	CreateEntities(ctx context.Context, entities []mcp.Entity) error
	GetEntity(ctx context.Context, id string) (*mcp.Entity, error)
	SearchEntities(ctx context.Context, query string) ([]mcp.Entity, error)
	GetStatistics(ctx context.Context) (map[string]int, error)
	Close() error
}