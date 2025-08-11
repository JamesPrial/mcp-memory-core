package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/internal/transport"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestServer_EdgeCases_HandleRequest_InvalidMethods(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)
	ctx := context.Background()

	// These should return -32600 Invalid Request (malformed)
	invalidRequestMethods := []string{
		"",  // Empty method
		"\x00tools/list",       // Null byte
		"tools/list\x00",       // Null byte
		"tools/list\n",         // Newline
		"tools/list\t",         // Tab
		"../tools/list",        // Contains ".."
		"tools/../list",        // Contains ".."
		"tools//list",          // Contains "//"
		strings.Repeat("a", 101), // Method name too long (>100 chars)
	}

	// These should return -32601 Method not found (valid format, unknown method)
	methodNotFoundMethods := []string{
		"invalid_method",
		"tools/invalid",
		"TOOLS/LIST",           // Wrong case
		"tools/list/extra",     // Extra path
		"tool/list",            // Missing 's'
		"tools-list",           // Wrong separator
		"tools.list",           // Wrong separator
		"tools\\list",          // Wrong separator
		"tools/call/extra",     // Extra path
		" tools/list ",         // Spaces (valid as a method name)
		"🦄tools/list",         // Unicode (valid as a method name)
		"tools/list🦄",         // Unicode (valid as a method name)
		"'; DROP TABLE tools; --", // SQL injection attempt (valid as a method name)
		"<script>alert('xss')</script>", // XSS attempt (valid as a method name)
	}

	for _, method := range invalidRequestMethods {
		t.Run(fmt.Sprintf("InvalidRequest_%d_chars", len(method)), func(t *testing.T) {
			req := &transport.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  method,
			}
			
			resp := server.HandleRequest(ctx, req)
			assert.Equal(t, "2.0", resp.JSONRPC)
			assert.Equal(t, req.ID, resp.ID)
			assert.Nil(t, resp.Result)
			assert.NotNil(t, resp.Error)
			assert.Equal(t, -32600, resp.Error.Code)
			assert.Equal(t, "Invalid Request", resp.Error.Message)
		})
	}

	for _, method := range methodNotFoundMethods {
		t.Run(fmt.Sprintf("MethodNotFound_%d_chars", len(method)), func(t *testing.T) {
			req := &transport.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  method,
			}
			
			resp := server.HandleRequest(ctx, req)
			assert.Equal(t, "2.0", resp.JSONRPC)
			assert.Equal(t, req.ID, resp.ID)
			assert.Nil(t, resp.Result)
			assert.NotNil(t, resp.Error)
			assert.Equal(t, -32601, resp.Error.Code)
			assert.Equal(t, "Method not found", resp.Error.Message)
		})
	}
}

func TestServer_EdgeCases_HandleRequest_InvalidRequestStructures(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)
	ctx := context.Background()

	t.Run("NilRequest", func(t *testing.T) {
		// Should panic or handle gracefully
		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer")
			}
		}()
		
		resp := server.HandleRequest(ctx, nil)
		if resp != nil {
			// If it returns a response instead of panicking, it should be an error
			assert.NotNil(t, resp.Error)
		}
	})

	t.Run("InvalidIDTypes", func(t *testing.T) {
		// JSON-RPC allows various ID types
		idTypes := []interface{}{
			nil,
			1,
			"string_id",
			1.5,
			true,
			[]interface{}{1, 2, 3},
			map[string]interface{}{"nested": "id"},
			strings.Repeat("a", 10000), // Very long ID
		}

		for i, id := range idTypes {
			t.Run(fmt.Sprintf("IDType_%d", i), func(t *testing.T) {
				req := &transport.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      id,
					Method:  "tools/list",
				}
				
				resp := server.HandleRequest(ctx, req)
				assert.Equal(t, "2.0", resp.JSONRPC)
				
				if id == nil {
					// nil ID should be rejected (notifications not supported)
					assert.Nil(t, resp.ID)
					assert.Nil(t, resp.Result)
					assert.NotNil(t, resp.Error)
					assert.Equal(t, -32600, resp.Error.Code)
				} else {
					// Other ID types should work
					assert.Equal(t, req.ID, resp.ID)
					assert.NotNil(t, resp.Result)
					assert.Nil(t, resp.Error)
				}
			})
		}
	})

	t.Run("InvalidJSONRPCVersions", func(t *testing.T) {
		// Server should handle various JSONRPC versions gracefully
		versions := []string{
			"",
			"1.0",
			"3.0",
			"2.1",
			"invalid",
			"\x00null",
			"🦄unicode",
		}

		for _, version := range versions {
			t.Run(fmt.Sprintf("Version_%s", version), func(t *testing.T) {
				req := &transport.JSONRPCRequest{
					JSONRPC: version,
					ID:      1,
					Method:  "tools/list",
				}
				
				resp := server.HandleRequest(ctx, req)
				// Response should always use "2.0" regardless of request version
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, req.ID, resp.ID)
			})
		}
	})
}

func TestServer_EdgeCases_HandleToolsList(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)

	t.Run("ValidRequest", func(t *testing.T) {
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}
		
		resp := server.handleToolsList(req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Result)
		
		result := resp.Result.(map[string]interface{})
		tools := result["tools"]
		assert.NotNil(t, tools)
	})

	t.Run("WithExtraParams", func(t *testing.T) {
		// tools/list should ignore any parameters
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "test_id",
			Method:  "tools/list",
			Params: map[string]interface{}{
				"unexpected": "parameter",
				"limit":      100,
				"filter":     "some_filter",
			},
		}
		
		resp := server.handleToolsList(req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, "test_id", resp.ID)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Result)
	})

	t.Run("NilParams", func(t *testing.T) {
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      nil,
			Method:  "tools/list",
			Params:  nil,
		}
		
		resp := server.handleToolsList(req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Nil(t, resp.ID)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Result)
	})
}

func TestServer_EdgeCases_HandleToolsCall_MalformedParams(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)
	ctx := context.Background()

	t.Run("MissingParams", func(t *testing.T) {
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			// No Params field
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32602, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Invalid params")
	})

	t.Run("NilParams", func(t *testing.T) {
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  nil,
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32602, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Invalid params")
	})

	t.Run("EmptyParams", func(t *testing.T) {
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  map[string]interface{}{},
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32602, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Invalid params")
	})

	t.Run("NameNotString", func(t *testing.T) {
		invalidNames := []interface{}{
			123,
			true,
			nil,
			[]string{"array", "name"},
			map[string]interface{}{"nested": "name"},
			1.5,
		}

		for i, name := range invalidNames {
			t.Run(fmt.Sprintf("InvalidName_%d", i), func(t *testing.T) {
				req := &transport.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      1,
					Method:  "tools/call",
					Params: map[string]interface{}{
						"name": name,
					},
				}
				
				resp := server.handleToolsCall(ctx, req)
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 1, resp.ID)
				assert.Nil(t, resp.Result)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, -32602, resp.Error.Code)
				assert.Contains(t, resp.Error.Message, "Invalid params")
			})
		}
	})

	t.Run("InvalidToolNames", func(t *testing.T) {
		// These fail validation and return -32602
		invalidParamsTools := []string{
			"",
			"unknown_tool",  // doesn't start with memory__
			strings.Repeat("a", 10000), // Very long name
			"\x00null_tool",
			"🦄unicode_tool",
			"'; DROP TABLE tools; --",
		}

		// This passes validation but tool doesn't exist, returns -32603
		unknownTools := []string{
			"memory__invalid",
		}

		for _, toolName := range invalidParamsTools {
			t.Run(fmt.Sprintf("InvalidParams_%d_chars", len(toolName)), func(t *testing.T) {
				req := &transport.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      1,
					Method:  "tools/call",
					Params: map[string]interface{}{
						"name": toolName,
					},
				}
				
				resp := server.handleToolsCall(ctx, req)
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 1, resp.ID)
				assert.Nil(t, resp.Result)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, -32602, resp.Error.Code)
				assert.Contains(t, resp.Error.Message, "Invalid params")
			})
		}

		for _, toolName := range unknownTools {
			t.Run(fmt.Sprintf("UnknownTool_%d_chars", len(toolName)), func(t *testing.T) {
				req := &transport.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      1,
					Method:  "tools/call",
					Params: map[string]interface{}{
						"name": toolName,
					},
				}
				
				resp := server.handleToolsCall(ctx, req)
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 1, resp.ID)
				assert.Nil(t, resp.Result)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, -32603, resp.Error.Code)
				assert.Contains(t, resp.Error.Message, "Internal error")
			})
		}
	})

	t.Run("MalformedArguments", func(t *testing.T) {
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		malformedArgs := []interface{}{
			"string_instead_of_map",
			123,
			true,
			[]string{"array", "args"},
			nil, // This should be handled as empty map
		}

		for i, args := range malformedArgs {
			t.Run(fmt.Sprintf("MalformedArgs_%d", i), func(t *testing.T) {
				req := &transport.JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      1,
					Method:  "tools/call",
					Params: map[string]interface{}{
						"name":      "memory__create_entities",
						"arguments": args,
					},
				}
				
				resp := server.handleToolsCall(ctx, req)
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 1, resp.ID)
				
				if args == nil {
					// nil arguments should be treated as empty map and cause tool-level error
					assert.NotNil(t, resp.Error)
					assert.Equal(t, -32603, resp.Error.Code)
				} else {
					// Non-map arguments should return invalid params error
					assert.NotNil(t, resp.Error)
					assert.Equal(t, -32602, resp.Error.Code)
				}
			})
		}
	})

	t.Run("ValidArgumentsIgnoredType", func(t *testing.T) {
		// When arguments is not a map[string]interface{}, it should be ignored
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name":      "memory__create_entities",
				"arguments": "not_a_map",
			},
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		// Should get invalid params error since arguments is not an object
		assert.Equal(t, -32602, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Invalid params")
	})

	t.Run("ExtraParamsIgnored", func(t *testing.T) {
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__create_entities",
				"arguments": map[string]interface{}{
					"entities": []interface{}{
						map[string]interface{}{"name": "Test Entity"},
					},
				},
				// Extra parameters that should be ignored
				"extra_param":  "ignored",
				"limit":        100,
				"malicious":    "'; DROP TABLE tools; --",
				"unicode":      "测试参数",
			},
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.NotNil(t, resp.Result)
		assert.Nil(t, resp.Error)
	})
}

func TestServer_EdgeCases_JSONSerialization(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)

	t.Run("ResponseSerialization", func(t *testing.T) {
		// Test that all response types can be serialized to JSON
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      map[string]interface{}{"complex": "id"}, // Complex ID
			Method:  "tools/list",
		}
		
		resp := server.HandleRequest(context.Background(), req)
		
		// Should be able to marshal to JSON without error
		jsonBytes, err := json.Marshal(resp)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)
		
		// Should be able to unmarshal back
		var unmarshaled transport.JSONRPCResponse
		err = json.Unmarshal(jsonBytes, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, resp.JSONRPC, unmarshaled.JSONRPC)
	})

	t.Run("ErrorResponseSerialization", func(t *testing.T) {
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "test",
			Method:  "invalid_method",
		}
		
		resp := server.HandleRequest(context.Background(), req)
		
		jsonBytes, err := json.Marshal(resp)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)
		
		// Verify error structure
		var unmarshaled transport.JSONRPCResponse
		err = json.Unmarshal(jsonBytes, &unmarshaled)
		assert.NoError(t, err)
		assert.NotNil(t, unmarshaled.Error)
		assert.Equal(t, -32601, unmarshaled.Error.Code)
	})
}

func TestServer_EdgeCases_ContextHandling(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)

	t.Run("CancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		mockStorage.On("SearchEntities", mock.Anything, "test").Return([]mcp.Entity{}, nil)

		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__search",
				"arguments": map[string]interface{}{
					"query": "test",
				},
			},
		}
		
		resp := server.HandleRequest(ctx, req)
		// Current implementation doesn't check context cancellation
		// This is a potential improvement area
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
	})

	t.Run("NilContext", func(t *testing.T) {
		// Should handle nil context gracefully
		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, fmt.Sprintf("%v", r), "nil")
			}
		}()

		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}
		
		resp := server.HandleRequest(nil, req)
		if resp != nil {
			assert.Equal(t, "2.0", resp.JSONRPC)
		}
	})
}

func TestServer_EdgeCases_ErrorHandling(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)
	ctx := context.Background()

	t.Run("ManagerError", func(t *testing.T) {
		// Test when manager returns an error
		mockStorage.On("SearchEntities", mock.Anything, "error_test").Return(nil, fmt.Errorf("search failed"))

		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__search",
				"arguments": map[string]interface{}{
					"query": "error_test",
				},
			},
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32603, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Internal error")
		assert.Contains(t, resp.Error.Message, "search failed")
	})

	t.Run("ErrorMessageSanitization", func(t *testing.T) {
		// Ensure error messages don't leak sensitive information
		mockStorage.On("SearchEntities", mock.Anything, "sensitive").Return(nil, 
			fmt.Errorf("database connection failed: password=secret123"))

		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__search",
				"arguments": map[string]interface{}{
					"query": "sensitive",
				},
			},
		}
		
		resp := server.handleToolsCall(ctx, req)
		assert.NotNil(t, resp.Error)
		// Error message should be sanitized
		assert.Equal(t, "Storage operation failed", resp.Error.Message)
		assert.NotContains(t, resp.Error.Message, "password=secret123")
		assert.NotContains(t, resp.Error.Message, "database connection failed")
	})
}

func TestServer_EdgeCases_BoundaryConditions(t *testing.T) {
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)
	ctx := context.Background()

	t.Run("VeryLargeRequest", func(t *testing.T) {
		// Create a request with very large data
		largeData := make([]interface{}, 1000)
		for i := 0; i < 1000; i++ {
			largeData[i] = map[string]interface{}{
				"name":         fmt.Sprintf("Entity %d", i),
				"observations": []string{strings.Repeat("data", 1000)}, // ~4KB per entity
			}
		}
		
		mockStorage.On("CreateEntities", mock.Anything, mock.AnythingOfType("[]mcp.Entity")).Return(nil)

		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__create_entities",
				"arguments": map[string]interface{}{
					"entities": largeData,
				},
			},
		}
		
		resp := server.HandleRequest(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		// Should handle large requests without errors
		if resp.Error != nil {
			// If there's an error, it shouldn't be due to size limitations
			assert.NotContains(t, resp.Error.Message, "too large")
			assert.NotContains(t, resp.Error.Message, "size limit")
		}
	})

	t.Run("DeepNestedParams", func(t *testing.T) {
		// Create deeply nested parameters
		nested := make(map[string]interface{})
		current := nested
		for i := 0; i < 100; i++ {
			next := make(map[string]interface{})
			current[fmt.Sprintf("level_%d", i)] = next
			current = next
		}
		current["final"] = "value"
		
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name":      "memory__search",
				"arguments": nested, // This won't be valid for search, but tests deep nesting
			},
		}
		
		resp := server.HandleRequest(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		// Should handle deep nesting without panicking
	})
}

func TestNewServer_EdgeCases(t *testing.T) {
	t.Run("NilManager", func(t *testing.T) {
		// NewServer should accept nil manager but handle it gracefully
		server := NewServer(nil)
		assert.NotNil(t, server)
		assert.Nil(t, server.manager)
		
		// Using the server with nil manager should return error responses
		ctx := context.Background()
		req := &transport.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}
		
		resp := server.HandleRequest(ctx, req)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32603, resp.Error.Code)
		assert.Equal(t, "Internal error", resp.Error.Message)
		assert.Equal(t, "Server not properly initialized", resp.Error.Data)
		
		// Test tools/call as well
		req.Method = "tools/call"
		req.Params = map[string]interface{}{
			"name": "memory__create_entities",
			"arguments": map[string]interface{}{},
		}
		
		resp = server.HandleRequest(ctx, req)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, -32603, resp.Error.Code)
		assert.Equal(t, "Internal error", resp.Error.Message)
		assert.Equal(t, "Server not properly initialized", resp.Error.Data)
	})
}