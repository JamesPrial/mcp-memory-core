// In file: internal/storage/backend_test.go
package storage

import (
	"context"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
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
	return args.Get(0).(*mcp.Entity), args.Error(1)
}

func (m *MockBackend) SearchEntities(ctx context.Context, query string) ([]mcp.Entity, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]mcp.Entity), args.Error(1)
}

func (m *MockBackend) GetStatistics(ctx context.Context) (map[string]int, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestInterfaceCompliance(t *testing.T) {
	var backend Backend = &MockBackend{}
	assert.NotNil(t, backend)
}

func TestMockBackend_GetEntity(t *testing.T) {
	mockBackend := new(MockBackend)
	expectedEntity := &mcp.Entity{ID: "e1", Name: "Test Entity", CreatedAt: time.Now()}

	mockBackend.On("GetEntity", mock.Anything, "e1").Return(expectedEntity, nil)

	entity, err := mockBackend.GetEntity(context.Background(), "e1")
	assert.NoError(t, err)
	assert.Equal(t, expectedEntity, entity)
	mockBackend.AssertExpectations(t)
}