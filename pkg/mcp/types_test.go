package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		value    EntityType
		expected string
	}{
		{"Person", EntityTypePerson, "Person"},
		{"Organization", EntityTypeOrganization, "Organization"},
		{"Project", EntityTypeProject, "Project"},
		{"Location", EntityTypeLocation, "Location"},
		{"Event", EntityTypeEvent, "Event"},
		{"Concept", EntityTypeConcept, "Concept"},
		{"Document", EntityTypeDocument, "Document"},
		{"General", EntityTypeGeneral, "General"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.value))
		})
	}
}

func TestEntity_JSONMarshaling(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second) // Truncate to avoid millisecond differences
	
	entity := Entity{
		ID:           "test-id",
		Name:         "Test Entity",
		EntityType:   EntityTypePerson,
		Observations: []string{"observation1", "observation2"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(entity)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"id":"test-id"`)
	assert.Contains(t, jsonStr, `"name":"Test Entity"`)
	assert.Contains(t, jsonStr, `"entityType":"Person"`)
	assert.Contains(t, jsonStr, `"observations":["observation1","observation2"]`)
	assert.Contains(t, jsonStr, `"createdAt"`)
	assert.Contains(t, jsonStr, `"updatedAt"`)

	// Unmarshal back to struct
	var unmarshaled Entity
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, entity.ID, unmarshaled.ID)
	assert.Equal(t, entity.Name, unmarshaled.Name)
	assert.Equal(t, entity.EntityType, unmarshaled.EntityType)
	assert.Equal(t, entity.Observations, unmarshaled.Observations)
	assert.Equal(t, entity.CreatedAt.UTC(), unmarshaled.CreatedAt.UTC())
	assert.Equal(t, entity.UpdatedAt.UTC(), unmarshaled.UpdatedAt.UTC())
}

func TestEntity_BackwardCompatibility(t *testing.T) {
	// Test that string entity types still work via JSON unmarshaling
	jsonData := `{
		"id": "compat-test",
		"name": "Backward Compatible Entity",
		"entityType": "custom_type",
		"observations": ["test"],
		"createdAt": "2023-01-01T00:00:00Z",
		"updatedAt": "2023-01-01T00:00:00Z"
	}`

	var entity Entity
	err := json.Unmarshal([]byte(jsonData), &entity)
	require.NoError(t, err)

	assert.Equal(t, "compat-test", entity.ID)
	assert.Equal(t, "Backward Compatible Entity", entity.Name)
	assert.Equal(t, EntityType("custom_type"), entity.EntityType)
	assert.Equal(t, []string{"test"}, entity.Observations)
}

func TestRelation_JSONMarshaling(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	
	relation := Relation{
		ID:          "rel-id",
		SourceID:    "source-id",
		TargetID:    "target-id",
		Description: "test relationship",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(relation)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"id":"rel-id"`)
	assert.Contains(t, jsonStr, `"sourceId":"source-id"`)
	assert.Contains(t, jsonStr, `"targetId":"target-id"`)
	assert.Contains(t, jsonStr, `"description":"test relationship"`)
	assert.Contains(t, jsonStr, `"createdAt"`)
	assert.Contains(t, jsonStr, `"updatedAt"`)

	// Unmarshal back to struct
	var unmarshaled Relation
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, relation.ID, unmarshaled.ID)
	assert.Equal(t, relation.SourceID, unmarshaled.SourceID)
	assert.Equal(t, relation.TargetID, unmarshaled.TargetID)
	assert.Equal(t, relation.Description, unmarshaled.Description)
	assert.Equal(t, relation.CreatedAt.UTC(), unmarshaled.CreatedAt.UTC())
	assert.Equal(t, relation.UpdatedAt.UTC(), unmarshaled.UpdatedAt.UTC())
}

func TestEntityType_TypeSafety(t *testing.T) {
	// Test that EntityType provides compile-time type safety
	entity := Entity{
		EntityType: EntityTypePerson, // This should compile
	}
	
	assert.Equal(t, EntityTypePerson, entity.EntityType)
	
	// Test string conversion
	assert.Equal(t, "Person", string(entity.EntityType))
	
	// Test that we can still assign string literals (for backward compatibility)
	var customType EntityType = "CustomType"
	entity.EntityType = customType
	assert.Equal(t, "CustomType", string(entity.EntityType))
}