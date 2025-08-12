package logging

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// AuditEvent represents a structured audit event
type AuditEvent struct {
	// Core fields
	Timestamp   time.Time   `json:"timestamp"`
	EventID     string      `json:"event_id"`
	EventType   string      `json:"event_type"`
	EventSource string      `json:"event_source"`
	
	// Request context
	RequestID   string      `json:"request_id,omitempty"`
	TraceID     string      `json:"trace_id,omitempty"`
	SessionID   string      `json:"session_id,omitempty"`
	
	// Actor information
	UserID      string      `json:"user_id,omitempty"`
	ActorIP     string      `json:"actor_ip,omitempty"`
	UserAgent   string      `json:"user_agent,omitempty"`
	
	// Resource information
	Resource    string      `json:"resource"`
	ResourceID  string      `json:"resource_id,omitempty"`
	Action      string      `json:"action"`
	
	// Operation results
	Result      string      `json:"result"`            // success, failure, error
	Duration    int64       `json:"duration_ms"`       // operation duration in ms
	
	// Data changes
	Changes     *Changes    `json:"changes,omitempty"` // before/after values
	
	// Additional context
	Details     map[string]interface{} `json:"details,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	
	// Security/compliance fields
	Severity    string      `json:"severity"`          // low, medium, high, critical
	Risk        string      `json:"risk,omitempty"`    // risk assessment
	Compliance  []string    `json:"compliance,omitempty"` // compliance frameworks
	
	// Tamper resistance
	Checksum    string      `json:"checksum"`          // integrity check
	Version     string      `json:"version"`           // audit log format version
}

// Changes represents before/after values for data modifications
type Changes struct {
	Before map[string]interface{} `json:"before,omitempty"`
	After  map[string]interface{} `json:"after,omitempty"`
	Fields []string               `json:"fields,omitempty"` // changed fields
}

// AuditLogger provides structured audit logging
type AuditLogger struct {
	file     *os.File
	encoder  *json.Encoder
	mu       sync.Mutex
	config   AuditConfig
	masker   *Masker
	metrics  *MetricsCollector
}

// AuditConfig defines audit logging configuration
type AuditConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	FilePath        string   `yaml:"filePath" json:"filePath"`
	MaxFileSize     int64    `yaml:"maxFileSize" json:"maxFileSize"`       // bytes
	MaxFiles        int      `yaml:"maxFiles" json:"maxFiles"`             // rotation count
	BufferSize      int      `yaml:"bufferSize" json:"bufferSize"`         // buffer entries
	FlushInterval   string   `yaml:"flushInterval" json:"flushInterval"`   // flush frequency
	CompressRotated bool     `yaml:"compressRotated" json:"compressRotated"`
	EnableMasking   bool     `yaml:"enableMasking" json:"enableMasking"`
	RequiredFields  []string `yaml:"requiredFields" json:"requiredFields"` // validation
	Retention       string   `yaml:"retention" json:"retention"`           // file retention period
}

// DefaultAuditConfig returns default audit configuration
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:         true,
		FilePath:        "audit.log",
		MaxFileSize:     100 * 1024 * 1024, // 100MB
		MaxFiles:        10,
		BufferSize:      1000,
		FlushInterval:   "5s",
		CompressRotated: true,
		EnableMasking:   true,
		RequiredFields:  []string{"timestamp", "event_type", "resource", "action", "result"},
		Retention:       "365d",
	}
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(filePath string) (*AuditLogger, error) {
	return NewAuditLoggerWithConfig(AuditConfig{
		Enabled:  true,
		FilePath: filePath,
	})
}

// NewAuditLoggerWithConfig creates a new audit logger with custom configuration
func NewAuditLoggerWithConfig(config AuditConfig) (*AuditLogger, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Apply defaults
	if config.FilePath == "" {
		config.FilePath = "audit.log"
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 100 * 1024 * 1024
	}
	if config.MaxFiles <= 0 {
		config.MaxFiles = 10
	}

	// Open audit log file
	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	logger := &AuditLogger{
		file:    file,
		encoder: json.NewEncoder(file),
		config:  config,
	}

	// Initialize masker if enabled
	if config.EnableMasking {
		maskingConfig := MaskingConfig{
			Enabled:          true,
			MaskEmails:       true,
			MaskPhoneNumbers: true,
			MaskCreditCards:  true,
			MaskSSN:          true,
			MaskAPIKeys:      true,
		}
		logger.masker = NewMasker(maskingConfig)
	}

	return logger, nil
}

// SetMetrics sets the metrics collector for the audit logger
func (al *AuditLogger) SetMetrics(metrics *MetricsCollector) {
	al.metrics = metrics
}

// Log logs an audit event
func (al *AuditLogger) Log(ctx context.Context, event AuditEvent) error {
	if al == nil {
		return nil
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	// Enrich event with context data
	event = al.enrichEvent(ctx, event)

	// Validate required fields
	if err := al.validateEvent(event); err != nil {
		return fmt.Errorf("audit event validation failed: %w", err)
	}

	// Apply masking if enabled
	if al.masker != nil {
		event = al.maskEvent(event)
	}

	// Calculate checksum for tamper resistance
	event.Checksum = al.calculateChecksum(event)
	event.Version = "1.0"

	// Write to file
	if err := al.encoder.Encode(event); err != nil {
		if al.metrics != nil {
			al.metrics.RecordLogError("audit", "audit")
		}
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	// Record metrics
	if al.metrics != nil {
		al.metrics.RecordAuditEvent(event.EventType, event.Result)
	}

	// Check if rotation is needed
	if err := al.checkRotation(); err != nil {
		// Log rotation error but don't fail the audit log
		fmt.Fprintf(os.Stderr, "audit log rotation failed: %v\n", err)
	}

	return nil
}

// enrichEvent adds context information to the audit event
func (al *AuditLogger) enrichEvent(ctx context.Context, event AuditEvent) AuditEvent {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Generate event ID if not provided
	if event.EventID == "" {
		event.EventID = GenerateID()
	}

	// Extract request context
	if event.RequestID == "" {
		event.RequestID = GetRequestID(ctx)
	}
	if event.TraceID == "" {
		event.TraceID = GetTraceID(ctx)
	}
	if event.SessionID == "" {
		event.SessionID = GetSessionID(ctx)
	}
	if event.UserID == "" {
		event.UserID = GetUserID(ctx)
	}

	// Set event source if not provided
	if event.EventSource == "" {
		event.EventSource = "mcp-memory-core"
	}

	// Set default severity if not provided
	if event.Severity == "" {
		event.Severity = al.determineSeverity(event)
	}

	return event
}

// validateEvent validates that required fields are present
func (al *AuditLogger) validateEvent(event AuditEvent) error {
	for _, field := range al.config.RequiredFields {
		switch field {
		case "timestamp":
			if event.Timestamp.IsZero() {
				return fmt.Errorf("required field 'timestamp' is missing")
			}
		case "event_type":
			if event.EventType == "" {
				return fmt.Errorf("required field 'event_type' is missing")
			}
		case "resource":
			if event.Resource == "" {
				return fmt.Errorf("required field 'resource' is missing")
			}
		case "action":
			if event.Action == "" {
				return fmt.Errorf("required field 'action' is missing")
			}
		case "result":
			if event.Result == "" {
				return fmt.Errorf("required field 'result' is missing")
			}
		}
	}
	return nil
}

// maskEvent applies data masking to the audit event
func (al *AuditLogger) maskEvent(event AuditEvent) AuditEvent {
	if al.masker == nil {
		return event
	}

	// Mask details
	if event.Details != nil {
		maskedDetails := make(map[string]interface{})
		for k, v := range event.Details {
			if str, ok := v.(string); ok {
				maskedDetails[k] = al.masker.MaskString(str)
			} else {
				maskedDetails[k] = v
			}
		}
		event.Details = maskedDetails
	}

	// Mask changes
	if event.Changes != nil {
		if event.Changes.Before != nil {
			maskedBefore := make(map[string]interface{})
			for k, v := range event.Changes.Before {
				if str, ok := v.(string); ok {
					maskedBefore[k] = al.masker.MaskString(str)
				} else {
					maskedBefore[k] = v
				}
			}
			event.Changes.Before = maskedBefore
		}

		if event.Changes.After != nil {
			maskedAfter := make(map[string]interface{})
			for k, v := range event.Changes.After {
				if str, ok := v.(string); ok {
					maskedAfter[k] = al.masker.MaskString(str)
				} else {
					maskedAfter[k] = v
				}
			}
			event.Changes.After = maskedAfter
		}
	}

	return event
}

// calculateChecksum calculates a SHA256 checksum for tamper resistance
func (al *AuditLogger) calculateChecksum(event AuditEvent) string {
	// Create a copy without the checksum field for hashing
	eventCopy := event
	eventCopy.Checksum = ""

	// Serialize to JSON for consistent hashing
	data, err := json.Marshal(eventCopy)
	if err != nil {
		return ""
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// determineSeverity determines the event severity based on event properties
func (al *AuditLogger) determineSeverity(event AuditEvent) string {
	// Critical events
	if event.Action == "delete" || event.Action == "destroy" {
		return "critical"
	}
	if event.Result == "failure" || event.Result == "error" {
		return "high"
	}

	// High risk events
	if event.Action == "create" || event.Action == "update" {
		return "medium"
	}

	// Default to low
	return "low"
}

// checkRotation checks if log rotation is needed
func (al *AuditLogger) checkRotation() error {
	stat, err := al.file.Stat()
	if err != nil {
		return err
	}

	if stat.Size() >= al.config.MaxFileSize {
		return al.rotate()
	}

	return nil
}

// rotate rotates the audit log file
func (al *AuditLogger) rotate() error {
	// Close current file
	if err := al.file.Close(); err != nil {
		return err
	}

	// Rotate files
	for i := al.config.MaxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", al.config.FilePath, i)
		newPath := fmt.Sprintf("%s.%d", al.config.FilePath, i+1)
		
		if _, err := os.Stat(oldPath); err == nil {
			if i == al.config.MaxFiles-1 {
				// Delete the oldest file
				os.Remove(oldPath)
			} else {
				os.Rename(oldPath, newPath)
			}
		}
	}

	// Move current file to .1
	rotatedPath := fmt.Sprintf("%s.1", al.config.FilePath)
	if err := os.Rename(al.config.FilePath, rotatedPath); err != nil {
		return err
	}

	// Compress rotated file if enabled
	if al.config.CompressRotated {
		go al.compressFile(rotatedPath)
	}

	// Create new file
	file, err := os.OpenFile(al.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	al.file = file
	al.encoder = json.NewEncoder(file)

	return nil
}

// compressFile compresses a rotated log file (placeholder implementation)
func (al *AuditLogger) compressFile(filePath string) {
	// TODO: Implement gzip compression
	// For now, this is a placeholder
}

// Close closes the audit logger
func (al *AuditLogger) Close() error {
	if al == nil || al.file == nil {
		return nil
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	err := al.file.Close()
	al.file = nil  // Mark as closed to prevent double close
	return err
}

// Sync forces a sync of the audit log to disk
func (al *AuditLogger) Sync() error {
	if al == nil || al.file == nil {
		return nil
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	return al.file.Sync()
}

// Helper functions for common audit events

// LogEntityCreated logs an entity creation event
func (al *AuditLogger) LogEntityCreated(ctx context.Context, entityType, entityID string, details map[string]interface{}) error {
	return al.Log(ctx, AuditEvent{
		EventType:  "entity.created",
		Resource:   "entity",
		ResourceID: entityID,
		Action:     "create",
		Result:     "success",
		Details: map[string]interface{}{
			"entity_type": entityType,
			"details":     details,
		},
		Tags: []string{"data_modification", "creation"},
	})
}

// LogEntityUpdated logs an entity update event
func (al *AuditLogger) LogEntityUpdated(ctx context.Context, entityType, entityID string, changes *Changes) error {
	return al.Log(ctx, AuditEvent{
		EventType:  "entity.updated",
		Resource:   "entity",
		ResourceID: entityID,
		Action:     "update",
		Result:     "success",
		Changes:    changes,
		Details: map[string]interface{}{
			"entity_type": entityType,
		},
		Tags: []string{"data_modification", "update"},
	})
}

// LogEntityDeleted logs an entity deletion event
func (al *AuditLogger) LogEntityDeleted(ctx context.Context, entityType, entityID string) error {
	return al.Log(ctx, AuditEvent{
		EventType:  "entity.deleted",
		Resource:   "entity",
		ResourceID: entityID,
		Action:     "delete",
		Result:     "success",
		Severity:   "critical",
		Details: map[string]interface{}{
			"entity_type": entityType,
		},
		Tags: []string{"data_modification", "deletion"},
	})
}

// LogRelationCreated logs a relation creation event
func (al *AuditLogger) LogRelationCreated(ctx context.Context, fromEntity, toEntity, relationType string) error {
	return al.Log(ctx, AuditEvent{
		EventType: "relation.created",
		Resource:  "relation",
		Action:    "create",
		Result:    "success",
		Details: map[string]interface{}{
			"from_entity":    fromEntity,
			"to_entity":      toEntity,
			"relation_type":  relationType,
		},
		Tags: []string{"data_modification", "relation"},
	})
}

// LogObservationAdded logs an observation addition event
func (al *AuditLogger) LogObservationAdded(ctx context.Context, entityID string, observation string) error {
	return al.Log(ctx, AuditEvent{
		EventType:  "observation.added",
		Resource:   "observation",
		ResourceID: entityID,
		Action:     "create",
		Result:     "success",
		Details: map[string]interface{}{
			"entity_id":   entityID,
			"observation": observation,
		},
		Tags: []string{"data_modification", "observation"},
	})
}

// LogAccessAttempt logs an access attempt event
func (al *AuditLogger) LogAccessAttempt(ctx context.Context, resource, action, result string, duration time.Duration) error {
	return al.Log(ctx, AuditEvent{
		EventType: "access.attempt",
		Resource:  resource,
		Action:    action,
		Result:    result,
		Duration:  duration.Milliseconds(),
		Severity:  func() string {
			if result == "failure" {
				return "high"
			}
			return "low"
		}(),
		Tags: []string{"access_control"},
	})
}

// LogSecurityEvent logs a security-related event
func (al *AuditLogger) LogSecurityEvent(ctx context.Context, eventType, description string, severity string) error {
	return al.Log(ctx, AuditEvent{
		EventType: eventType,
		Resource:  "security",
		Action:    "monitor",
		Result:    "detected",
		Severity:  severity,
		Details: map[string]interface{}{
			"description": description,
		},
		Tags: []string{"security", "monitoring"},
	})
}

// VerifyChecksum verifies the integrity of an audit event
func VerifyChecksum(event AuditEvent) bool {
	expectedChecksum := event.Checksum
	event.Checksum = ""

	data, err := json.Marshal(event)
	if err != nil {
		return false
	}

	hash := sha256.Sum256(data)
	actualChecksum := hex.EncodeToString(hash[:])

	return expectedChecksum == actualChecksum
}

// ReadAuditLog reads and parses audit log entries
func ReadAuditLog(reader io.Reader) ([]AuditEvent, error) {
	decoder := json.NewDecoder(reader)
	var events []AuditEvent

	for decoder.More() {
		var event AuditEvent
		if err := decoder.Decode(&event); err != nil {
			return nil, fmt.Errorf("failed to decode audit event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

// FilterAuditEvents filters audit events based on criteria
func FilterAuditEvents(events []AuditEvent, filter func(AuditEvent) bool) []AuditEvent {
	var filtered []AuditEvent
	for _, event := range events {
		if filter(event) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}