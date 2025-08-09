// In file: pkg/mcp/types.go
package mcp

import "time"

type Entity struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	EntityType  string    `json:"entityType"`
	Observations []string  `json:"observations"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Relation struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	TargetID    string    `json:"targetId"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}