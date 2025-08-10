package mcp

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntity_UUIDGeneration tests that entities can use UUIDs for IDs
func TestEntity_UUIDGeneration(t *testing.T) {
	// Test that we can use UUID for entity IDs
	entityID := uuid.New().String()
	
	entity := Entity{
		ID:         entityID,
		Name:       "UUID Entity",
		EntityType: EntityTypePerson,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Validate that the ID is a valid UUID
	parsedUUID, err := uuid.Parse(entity.ID)
	require.NoError(t, err)
	assert.Equal(t, entityID, parsedUUID.String())
}

// TestEntity_UUIDUniqueness tests that multiple UUID generations are unique
func TestEntity_UUIDUniqueness(t *testing.T) {
	const numEntities = 100
	ids := make(map[string]bool)
	
	for i := 0; i < numEntities; i++ {
		entityID := uuid.New().String()
		
		// Ensure no duplicates
		assert.False(t, ids[entityID], "UUID should be unique: %s", entityID)
		ids[entityID] = true
		
		// Validate UUID format
		_, err := uuid.Parse(entityID)
		require.NoError(t, err, "Generated ID should be valid UUID: %s", entityID)
	}
	
	assert.Len(t, ids, numEntities, "Should have generated %d unique UUIDs", numEntities)
}

// TestEntity_UpdatedAtField tests the UpdatedAt field functionality
func TestEntity_UpdatedAtField(t *testing.T) {
	// Test that UpdatedAt can be set and modified
	createdTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedTime := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)
	
	entity := Entity{
		ID:         uuid.New().String(),
		Name:       "Update Test",
		EntityType: EntityTypePerson,
		CreatedAt:  createdTime,
		UpdatedAt:  createdTime, // Initially same as CreatedAt
	}

	// Simulate an update
	entity.UpdatedAt = updatedTime
	entity.Name = "Updated Name"

	assert.True(t, entity.UpdatedAt.After(entity.CreatedAt))
	assert.Equal(t, "Updated Name", entity.Name)
	assert.Equal(t, updatedTime, entity.UpdatedAt)
}

// TestRelation_UUIDGeneration tests UUID usage in relations
func TestRelation_UUIDGeneration(t *testing.T) {
	// Test that we can use UUIDs for relation IDs and references
	relationID := uuid.New().String()
	sourceID := uuid.New().String()
	targetID := uuid.New().String()
	
	relation := Relation{
		ID:          relationID,
		SourceID:    sourceID,
		TargetID:    targetID,
		Description: "related to",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Validate that all IDs are valid UUIDs
	_, err := uuid.Parse(relation.ID)
	require.NoError(t, err, "Relation ID should be valid UUID")
	_, err = uuid.Parse(relation.SourceID)
	require.NoError(t, err, "Source ID should be valid UUID")
	_, err = uuid.Parse(relation.TargetID)
	require.NoError(t, err, "Target ID should be valid UUID")
}

// TestRelation_UpdatedAtField tests the UpdatedAt field in relations
func TestRelation_UpdatedAtField(t *testing.T) {
	// Test that Relation UpdatedAt can be set and modified
	createdTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedTime := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)
	
	relation := Relation{
		ID:          uuid.New().String(),
		SourceID:    uuid.New().String(),
		TargetID:    uuid.New().String(),
		Description: "initial description",
		CreatedAt:   createdTime,
		UpdatedAt:   createdTime, // Initially same as CreatedAt
	}

	// Simulate an update
	relation.UpdatedAt = updatedTime
	relation.Description = "updated description"

	assert.True(t, relation.UpdatedAt.After(relation.CreatedAt))
	assert.Equal(t, "updated description", relation.Description)
	assert.Equal(t, updatedTime, relation.UpdatedAt)
}

// TestEntity_TimeFieldsConsistency tests time field consistency and validation
func TestEntity_TimeFieldsConsistency(t *testing.T) {
	baseTime := time.Date(2023, 12, 25, 12, 0, 0, 0, time.UTC)
	
	tests := []struct {
		name        string
		createdAt   time.Time
		updatedAt   time.Time
		expectValid bool
	}{
		{
			name:        "same time is valid",
			createdAt:   baseTime,
			updatedAt:   baseTime,
			expectValid: true,
		},
		{
			name:        "updated after created is valid",
			createdAt:   baseTime,
			updatedAt:   baseTime.Add(time.Hour),
			expectValid: true,
		},
		{
			name:        "zero times are valid",
			createdAt:   time.Time{},
			updatedAt:   time.Time{},
			expectValid: true,
		},
		{
			name:        "updated before created is unusual but allowed",
			createdAt:   baseTime,
			updatedAt:   baseTime.Add(-time.Hour),
			expectValid: true, // System allows this, though it's unusual
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := Entity{
				ID:         uuid.New().String(),
				Name:       "Time Test Entity",
				EntityType: EntityTypeGeneral,
				CreatedAt:  tt.createdAt,
				UpdatedAt:  tt.updatedAt,
			}

			// All time combinations are valid in the data structure
			assert.Equal(t, tt.createdAt, entity.CreatedAt)
			assert.Equal(t, tt.updatedAt, entity.UpdatedAt)
			
			// Business logic validation could be added here if needed
			if tt.expectValid && !tt.updatedAt.IsZero() && !tt.createdAt.IsZero() {
				// This is where business rules could be enforced
				if tt.name == "updated after created is valid" {
					assert.True(t, entity.UpdatedAt.After(entity.CreatedAt) || entity.UpdatedAt.Equal(entity.CreatedAt))
				}
			}
		})
	}
}

// BenchmarkUUIDGeneration tests the performance of UUID generation
func BenchmarkUUIDGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uuid.New().String()
	}
}

// BenchmarkEntityWithUUID benchmarks entity creation with UUID
func BenchmarkEntityWithUUID(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		entity := Entity{
			ID:           uuid.New().String(),
			Name:         "Benchmark Entity",
			EntityType:   EntityTypePerson,
			Observations: []string{"observation1", "observation2"},
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		_ = entity // Prevent compiler optimization
	}
}