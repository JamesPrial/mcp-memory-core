package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/internal/transport"
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)


// Server represents the MCP server
type Server struct {
	manager *knowledge.Manager
}

// NewServer creates a new MCP server
func NewServer(manager *knowledge.Manager) *Server {
	return &Server{
		manager: manager,
	}
}

// validateRequest validates a JSON-RPC request and returns an error response if invalid
func (s *Server) validateRequest(req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	// Check if request is nil
	if req == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Request cannot be null",
			},
		}
	}

	// Validate jsonrpc field
	if req.JSONRPC != "2.0" {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Invalid or missing 'jsonrpc' field, must be '2.0'",
			},
		}
	}

	// Validate method field
	if req.Method == "" {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Missing or empty 'method' field",
			},
		}
	}

	// ID field validation - it can be string, number, or null, but not missing entirely
	// In JSON-RPC 2.0, notifications don't have ID, but regular requests must have one
	if req.ID == nil {
		// This could be a notification, but for MCP we require IDs
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Missing 'id' field - notifications are not supported",
			},
		}
	}

	// Check for invalid method patterns
	if len(req.Method) > 100 || strings.Contains(req.Method, "..") || strings.Contains(req.Method, "//") ||
		strings.Contains(req.Method, "\x00") || strings.Contains(req.Method, "\n") || strings.Contains(req.Method, "\t") {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    fmt.Sprintf("Invalid method format: '%s'", req.Method),
			},
		}
	}

	return nil // Request is valid
}

// isValidToolName validates that a tool name is properly formatted
func isValidToolName(name string) bool {
	// Tool name must not be empty and should have reasonable length
	if name == "" || len(name) > 100 {
		return false
	}
	
	// Tool name should only contain lowercase alphanumeric characters, underscores, and hyphens
	// No special characters, unicode, spaces, etc.
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || 
			 (r >= 'A' && r <= 'Z') ||
			 (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	
	return true
}


// sanitizeError converts internal errors to user-safe messages
func (s *Server) sanitizeError(err error) string {
	if err == nil {
		return "Unknown error occurred"
	}

	errMsg := strings.ToLower(err.Error())
	
	// Replace internal error patterns with user-friendly messages
	if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "sql") {
		return "Storage operation failed"
	}
	if strings.Contains(errMsg, "memory") || strings.Contains(errMsg, "allocation") {
		return "Memory operation failed"
	}
	if strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "access") {
		return "Access denied"
	}
	if strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network") {
		return "Connection error"
	}
	if strings.Contains(errMsg, "timeout") {
		return "Operation timed out"
	}
	if strings.Contains(errMsg, "not found") {
		return "Requested resource not found"
	}
	if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "malformed") {
		return "Invalid input provided"
	}

	// For any other errors, return a generic message
	return "Internal server error"
}

// HandleRequest processes a JSON-RPC request and returns a response
func (s *Server) HandleRequest(ctx context.Context, req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	// First, validate the request
	if validationErr := s.validateRequest(req); validationErr != nil {
		return validationErr
	}


	switch req.Method {
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		// This is -32601 Method not found (for valid format but unknown method)
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
				Data:    fmt.Sprintf("Method '%s' is not supported", req.Method),
			},
		}
	}
}

// handleToolsList handles the tools/list method
func (s *Server) handleToolsList(req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in handleToolsList: %v", r)
		}
	}()

	// Check if manager is nil
	if s.manager == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "Server not properly initialized",
			},
		}
	}

	tools := s.manager.HandleListTools()
	
	return &transport.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleToolsCall handles the tools/call method
func (s *Server) handleToolsCall(ctx context.Context, req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in handleToolsCall: %v", r)
		}
	}()

	// Check if manager is nil
	if s.manager == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "Server not properly initialized",
			},
		}
	}

	// Validate params exists
	if req.Params == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing 'params' field for tools/call method",
			},
		}
	}

	// Extract tool name from params
	name, ok := req.Params["name"]
	if !ok {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing 'name' field in params",
			},
		}
	}

	toolName, ok := name.(string)
	if !ok {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' must be a string",
			},
		}
	}

	if toolName == "" {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' cannot be empty",
			},
		}
	}
	
	// Check arguments format first (before validating tool name)
	// This ensures we give consistent error messages for malformed arguments
	if args, exists := req.Params["arguments"]; exists {
		// nil arguments should be treated as empty map
		if args != nil {
			// Non-nil arguments must be a map
			if _, ok := args.(map[string]interface{}); !ok {
				// Non-nil, non-map arguments are invalid
				return &transport.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &transport.JSONRPCError{
						Code:    -32602,
						Message: "Invalid params",
						Data:    "Field 'arguments' must be an object",
					},
				}
			}
		}
	}
	
	// Validate tool name format - should contain only alphanumeric, underscores
	// Tool names shouldn't have special characters, unicode, etc.
	if !isValidToolName(toolName) {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' contains invalid characters or unknown tool",
			},
		}
	}
	
	// Check if the tool exists (basic validation for known patterns)
	if !strings.HasPrefix(toolName, "memory__") {
		// Tool doesn't match expected pattern
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Unknown tool name",
			},
		}
	}

	// Extract arguments from params (we've already validated the format above)
	arguments := make(map[string]interface{})
	if args, exists := req.Params["arguments"]; exists {
		// nil arguments should be treated as empty map
		if args != nil {
			if argsMap, ok := args.(map[string]interface{}); ok {
				arguments = argsMap
			}
			// We already validated above, so this should always succeed
		}
	}

	// Call the tool
	result, err := s.manager.HandleCallTool(ctx, toolName, arguments)
	if err != nil {
		// Check if error needs sanitization
		errMsg := err.Error()
		needsSanitization := strings.Contains(strings.ToLower(errMsg), "password") ||
			strings.Contains(strings.ToLower(errMsg), "secret") ||
			strings.Contains(strings.ToLower(errMsg), "token") ||
			strings.Contains(strings.ToLower(errMsg), "key") ||
			strings.Contains(strings.ToLower(errMsg), "database connection")
		
		var message string
		if needsSanitization {
			message = s.sanitizeError(err)
		} else {
			message = fmt.Sprintf("Internal error: %s", err.Error())
		}
		
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32603,
				Message: message,
				Data:    nil,
			},
		}
	}

	return &transport.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}


func main() {
	// Parse command-line flags
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage backend
	backend, err := storage.NewBackend(cfg)
	if err != nil {
		log.Fatalf("Failed to create storage backend: %v", err)
	}
	defer backend.Close()

	// Create knowledge manager
	manager := knowledge.NewManager(backend)

	// Create server
	server := NewServer(manager)
	
	// Create transport based on configuration
	factory := transport.NewFactory()
	trans, err := factory.CreateTransport(cfg)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}
	
	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		cancel()
	}()
	
	// Start the transport with request handler
	log.Printf("Starting %s transport...", trans.Name())
	if err := trans.Start(ctx, server.HandleRequest); err != nil {
		if err != context.Canceled {
			log.Fatalf("Transport error: %v", err)
		}
	}
	
	log.Println("Server stopped")
}