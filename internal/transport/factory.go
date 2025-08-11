package transport

import (
	"fmt"
	
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

// Factory creates transport instances based on configuration
type Factory struct{}

// NewFactory creates a new transport factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateTransport creates a transport instance based on the configuration
func (f *Factory) CreateTransport(cfg *config.Settings) (Transport, error) {
	switch cfg.TransportType {
	case "stdio", "":
		return NewStdioTransport(), nil
		
	case "http":
		return NewHTTPTransport(&cfg.Transport), nil
		
	case "sse":
		return NewSSETransport(&cfg.Transport), nil
		
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", cfg.TransportType)
	}
}