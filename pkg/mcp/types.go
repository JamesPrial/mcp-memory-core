package mcp

import "time"

// EntityType represents the type of an entity with predefined constants for type safety
type EntityType string

const (
	EntityTypePerson       EntityType = "Person"
	EntityTypeOrganization EntityType = "Organization"
	EntityTypeProject      EntityType = "Project"
	EntityTypeLocation     EntityType = "Location"
	EntityTypeEvent        EntityType = "Event"
	EntityTypeConcept      EntityType = "Concept"
	EntityTypeDocument     EntityType = "Document"
	EntityTypeGeneral      EntityType = "General"
)

type Entity struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	EntityType   EntityType `json:"entityType"`
	Observations []string   `json:"observations"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Relation struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	TargetID    string    `json:"targetId"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Error represents a JSON-RPC error response compatible with MCP transport layer
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}