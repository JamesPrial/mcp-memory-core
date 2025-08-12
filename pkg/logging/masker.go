package logging

import (
	"log/slog"
	"regexp"
	"strings"
)

// Masker provides sensitive data masking functionality
type Masker struct {
	config   MaskingConfig
	patterns []*regexp.Regexp
}

// NewMasker creates a new masker
func NewMasker(config MaskingConfig) *Masker {
	m := &Masker{
		config:   config,
		patterns: make([]*regexp.Regexp, 0),
	}
	
	// Compile custom patterns
	for _, pattern := range config.Patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			m.patterns = append(m.patterns, re)
		}
	}
	
	// Add predefined patterns
	if config.MaskEmails {
		if re, err := regexp.Compile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`); err == nil {
			m.patterns = append(m.patterns, re)
		}
	}
	
	if config.MaskPhoneNumbers {
		if re, err := regexp.Compile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`); err == nil {
			m.patterns = append(m.patterns, re)
		}
	}
	
	if config.MaskCreditCards {
		if re, err := regexp.Compile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`); err == nil {
			m.patterns = append(m.patterns, re)
		}
	}
	
	if config.MaskSSN {
		if re, err := regexp.Compile(`\b\d{3}-?\d{2}-?\d{4}\b`); err == nil {
			m.patterns = append(m.patterns, re)
		}
	}
	
	if config.MaskAPIKeys {
		if re, err := regexp.Compile(`(?i)\b[a-z0-9]{32,}\b`); err == nil {
			m.patterns = append(m.patterns, re)
		}
	}
	
	return m
}

// MaskAttr masks sensitive data in a log attribute
func (m *Masker) MaskAttr(groups []string, attr slog.Attr) slog.Attr {
	if !m.config.Enabled {
		return attr
	}
	
	// Check if field should be masked
	if m.shouldMaskField(attr.Key) {
		return slog.Attr{
			Key:   attr.Key,
			Value: slog.StringValue("***MASKED***"),
		}
	}
	
	// Mask string values
	if attr.Value.Kind() == slog.KindString {
		masked := m.MaskString(attr.Value.String())
		return slog.Attr{
			Key:   attr.Key,
			Value: slog.StringValue(masked),
		}
	}
	
	return attr
}

// shouldMaskField checks if a field should be completely masked
func (m *Masker) shouldMaskField(field string) bool {
	fieldLower := strings.ToLower(field)
	
	for _, maskField := range m.config.Fields {
		if strings.ToLower(maskField) == fieldLower {
			return true
		}
	}
	
	// Common sensitive field names
	sensitiveFields := []string{
		"password", "passwd", "pwd", "secret", "token", "key", "auth",
		"authorization", "credential", "private", "confidential",
	}
	
	for _, sensitive := range sensitiveFields {
		if strings.Contains(fieldLower, sensitive) {
			return true
		}
	}
	
	return false
}

// MaskString masks sensitive patterns in a string
func (m *Masker) MaskString(s string) string {
	masked := s
	
	for _, pattern := range m.patterns {
		masked = pattern.ReplaceAllStringFunc(masked, func(match string) string {
			if len(match) <= 4 {
				return "***"
			}
			// Show first and last characters, mask the middle
			return match[:2] + strings.Repeat("*", len(match)-4) + match[len(match)-2:]
		})
	}
	
	return masked
}