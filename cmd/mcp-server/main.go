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

// HandleRequest processes a JSON-RPC request and returns a response
func (s *Server) HandleRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

// handleToolsList handles the tools/list method
func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
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
	// Extract tool name from params
	name, ok := req.Params["name"]
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params: name is required",
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
				Message: "Invalid params: name must be a string",
			},
		}
	}

	// Extract arguments from params
	arguments := make(map[string]interface{})
	if args, exists := req.Params["arguments"]; exists {
		if argsMap, ok := args.(map[string]interface{}); ok {
			arguments = argsMap
		}
	}

	// Call the tool
	result, err := s.manager.HandleCallTool(ctx, toolName, arguments)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32603,
				Message: fmt.Sprintf("Internal error: %s", err.Error()),
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