package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

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
func (s *Server) validateRequest(req *JSONRPCRequest) *JSONRPCResponse {
	// Check if request is nil
	if req == nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Request cannot be null",
			},
		}
	}

	// Validate jsonrpc field
	if req.JSONRPC != "2.0" {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Invalid or missing 'jsonrpc' field, must be '2.0'",
			},
		}
	}

	// Validate method field
	if req.Method == "" {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
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
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Missing 'id' field - notifications are not supported",
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

// isValidMethodName validates that a method name is properly formatted
func isValidMethodName(method string) bool {
	// Method must not be empty and should have reasonable length
	if method == "" || len(method) > 100 {
		return false
	}
	
	// Method should only contain lowercase alphanumeric characters, forward slashes, and underscores
	// Must be all lowercase to prevent case variations
	// No dots, hyphens, or other special characters allowed
	for _, r := range method {
		if !((r >= 'a' && r <= 'z') || 
			 (r >= '0' && r <= '9') || r == '/' || r == '_') {
			return false
		}
	}
	
	// Check for path traversal attempts
	if strings.Contains(method, "..") || strings.Contains(method, "//") {
		return false
	}
	
	// Method should not have extra path segments beyond expected pattern
	if strings.Count(method, "/") > 1 {
		return false
	}
	
	// Only allow specific known methods
	if method != "tools/list" && method != "tools/call" {
		return false
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
func (s *Server) HandleRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	// First, validate the request
	if validationErr := s.validateRequest(req); validationErr != nil {
		return validationErr
	}

	// Check for malformed method names (containing invalid characters, etc.)
	// This is -32600 Invalid Request
	for _, r := range req.Method {
		if !((r >= 'a' && r <= 'z') || 
			 (r >= 'A' && r <= 'Z') ||
			 (r >= '0' && r <= '9') || r == '/' || r == '_' || r == '-' || r == '.' || r == '\\') {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &JSONRPCError{
					Code:    -32600,
					Message: "Invalid Request",
					Data:    fmt.Sprintf("Invalid method format: '%s'", req.Method),
				},
			}
		}
	}
	
	// Check for other invalid patterns
	if len(req.Method) > 100 || strings.Contains(req.Method, "..") || strings.Contains(req.Method, "//") ||
		strings.Contains(req.Method, "\x00") || strings.Contains(req.Method, "\n") || strings.Contains(req.Method, "\t") {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    fmt.Sprintf("Invalid method format: '%s'", req.Method),
			},
		}
	}

	switch req.Method {
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		// This is -32601 Method not found (for valid format but unknown method)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
				Data:    fmt.Sprintf("Method '%s' is not supported", req.Method),
			},
		}
	}
}

// handleToolsList handles the tools/list method
func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in handleToolsList: %v", r)
		}
	}()

	tools := s.manager.HandleListTools()
	
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleToolsCall handles the tools/call method
func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in handleToolsCall: %v", r)
		}
	}()

	// Validate params exists
	if req.Params == nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing 'params' field for tools/call method",
			},
		}
	}

	// Extract tool name from params
	name, ok := req.Params["name"]
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing 'name' field in params",
			},
		}
	}

	toolName, ok := name.(string)
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' must be a string",
			},
		}
	}

	if toolName == "" {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
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
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &JSONRPCError{
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
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' contains invalid characters or unknown tool",
			},
		}
	}
	
	// Check if the tool exists (basic validation for known patterns)
	if !strings.HasPrefix(toolName, "memory__") {
		// Tool doesn't match expected pattern
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
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
		
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32603,
				Message: message,
				Data:    nil,
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// Run starts the stdio JSON-RPC server
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Send parse error response
			resp := &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error",
					Data:    "Invalid JSON format",
				},
			}
			s.sendResponse(resp)
			continue
		}

		// Handle the request
		resp := s.HandleRequest(ctx, &req)
		s.sendResponse(resp)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading from stdin: %w", err)
	}

	return nil
}

// sendResponse sends a JSON-RPC response to stdout
func (s *Server) sendResponse(resp *JSONRPCResponse) {
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		// Send a generic error response if marshaling fails
		fallbackResp := &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "Failed to serialize response",
			},
		}
		if fallbackBytes, fallbackErr := json.Marshal(fallbackResp); fallbackErr == nil {
			fmt.Println(string(fallbackBytes))
		}
		return
	}
	
	fmt.Println(string(respBytes))
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

	// Create and start server
	server := NewServer(manager)
	
	ctx := context.Background()
	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}