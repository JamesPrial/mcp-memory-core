package storage

import (
	"context"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/mock"
)

type MockBackend struct {
	mock.Mock
}

func (m *MockBackend) CreateEntities(ctx context.Context, entities []mcp.Entity) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockBackend) GetEntity(ctx context.Context, id string) (*mcp.Entity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.Entity), args.Error(1)
}

func (m *MockBackend) SearchEntities(ctx context.Context, query string) ([]mcp.Entity, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]mcp.Entity), args.Error(1)
}

func (m *MockBackend) GetStatistics(ctx context.Context) (map[string]int, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}