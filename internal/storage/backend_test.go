// In file: internal/storage/backend_test.go
package storage

import (
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInterfaceCompliance(t *testing.T) {
	var backend Backend = &MockBackend{}
	assert.NotNil(t, backend)
}

func TestMockBackend_GetEntity(t *testing.T) {
	mockBackend := new(MockBackend)
	expectedEntity := &mcp.Entity{ID: "e1", Name: "Test Entity", CreatedAt: time.Now()}

	mockBackend.On("GetEntity", mock.Anything, "e1").Return(expectedEntity, nil)

	entity, err := mockBackend.GetEntity(mock.Anything, "e1")
	assert.NoError(t, err)
	assert.Equal(t, expectedEntity, entity)
	mockBackend.AssertExpectations(t)
}