// In file: cmd/mcp-server/main_test.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdioIntegration(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "test-server")
	err := cmd.Run()
	require.NoError(t, err, "Failed to build server binary")

	serverCmd := exec.Command("./test-server")
	stdin, err := serverCmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := serverCmd.StdoutPipe()
	require.NoError(t, err)

	err = serverCmd.Start()
	require.NoError(t, err)
	defer serverCmd.Process.Kill()

	reader := bufio.NewReader(stdout)

	listReq := map[string]interface{} {
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	reqBytes, _ := json.Marshal(listReq)
	stdin.Write(append(reqBytes, '\n'))

	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var listResp map[string]interface{}
	err = json.Unmarshal(line, &listResp)
	require.NoError(t, err)

	assert.Equal(t, float64(1), listResp["id"])
	assert.NotNil(t, listResp["result"])
}

func TestHandleRequest_NilRequest(t *testing.T) {
	server := &Server{}
	
	resp := server.HandleRequest(context.Background(), nil)
	
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32600, resp.Error.Code)
	assert.Equal(t, "Invalid Request", resp.Error.Message)
	assert.Equal(t, "Request cannot be null", resp.Error.Data)
}

func TestHandleRequest_ValidationErrors(t *testing.T) {
	server := &Server{}
	
	tests := []struct {
		name           string
		request        *JSONRPCRequest
		expectedCode   int
		expectedMsg    string
		expectedData   string
	}{
		{
			name: "missing jsonrpc field",
			request: &JSONRPCRequest{
				ID:     1,
				Method: "tools/list",
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request",
			expectedData: "Invalid or missing 'jsonrpc' field, must be '2.0'",
		},
		{
			name: "invalid jsonrpc version",
			request: &JSONRPCRequest{
				JSONRPC: "1.0",
				ID:      1,
				Method:  "tools/list",
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request",
			expectedData: "Invalid or missing 'jsonrpc' field, must be '2.0'",
		},
		{
			name: "empty method field",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "",
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request",
			expectedData: "Missing or empty 'method' field",
		},
		{
			name: "missing method field",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request",
			expectedData: "Missing or empty 'method' field",
		},
		{
			name: "missing id field",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "tools/list",
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request",
			expectedData: "Missing 'id' field - notifications are not supported",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := server.HandleRequest(context.Background(), tt.request)
			
			require.NotNil(t, resp)
			assert.Equal(t, "2.0", resp.JSONRPC)
			assert.NotNil(t, resp.Error)
			assert.Equal(t, tt.expectedCode, resp.Error.Code)
			assert.Equal(t, tt.expectedMsg, resp.Error.Message)
			assert.Equal(t, tt.expectedData, resp.Error.Data)
		})
	}
}

func TestHandleRequest_MethodNotFound(t *testing.T) {
	server := &Server{}
	
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "invalid/method",
	}
	
	resp := server.HandleRequest(context.Background(), req)
	
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)
	assert.Equal(t, "Method 'invalid/method' is not supported", resp.Error.Data)
}

func TestHandleToolsCall_ValidationErrors(t *testing.T) {
	// Create a server with a mock manager
	mockStorage := new(storage.MockBackend)
	manager := knowledge.NewManager(mockStorage)
	server := NewServer(manager)
	
	tests := []struct {
		name           string
		request        *JSONRPCRequest
		expectedCode   int
		expectedMsg    string
		expectedData   string
	}{
		{
			name: "missing params",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
			},
			expectedCode: -32602,
			expectedMsg:  "Invalid params",
			expectedData: "Missing 'params' field for tools/call method",
		},
		{
			name: "missing name in params",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params:  map[string]interface{}{},
			},
			expectedCode: -32602,
			expectedMsg:  "Invalid params",
			expectedData: "Missing 'name' field in params",
		},
		{
			name: "name not a string",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": 123,
				},
			},
			expectedCode: -32602,
			expectedMsg:  "Invalid params",
			expectedData: "Field 'name' must be a string",
		},
		{
			name: "empty name",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "",
				},
			},
			expectedCode: -32602,
			expectedMsg:  "Invalid params",
			expectedData: "Field 'name' cannot be empty",
		},
		{
			name: "arguments not an object",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "test-tool",
					"arguments": "invalid",
				},
			},
			expectedCode: -32602,
			expectedMsg:  "Invalid params",
			expectedData: "Field 'arguments' must be an object",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := server.HandleRequest(context.Background(), tt.request)
			
			require.NotNil(t, resp)
			assert.Equal(t, "2.0", resp.JSONRPC)
			assert.Equal(t, 1, resp.ID)
			assert.NotNil(t, resp.Error)
			assert.Equal(t, tt.expectedCode, resp.Error.Code)
			assert.Equal(t, tt.expectedMsg, resp.Error.Message)
			assert.Equal(t, tt.expectedData, resp.Error.Data)
		})
	}
}

func TestSanitizeError(t *testing.T) {
	server := &Server{}
	
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: "Unknown error occurred",
		},
		{
			name:     "database error",
			input:    errors.New("database connection failed"),
			expected: "Storage operation failed",
		},
		{
			name:     "sql error",
			input:    errors.New("sql syntax error in query"),
			expected: "Storage operation failed",
		},
		{
			name:     "memory error",
			input:    errors.New("memory allocation failed"),
			expected: "Memory operation failed",
		},
		{
			name:     "permission error",
			input:    errors.New("permission denied to access file"),
			expected: "Access denied",
		},
		{
			name:     "access error",
			input:    errors.New("access forbidden"),
			expected: "Access denied",
		},
		{
			name:     "connection error",
			input:    errors.New("connection timed out"),
			expected: "Connection error",
		},
		{
			name:     "network error",
			input:    errors.New("network unreachable"),
			expected: "Connection error",
		},
		{
			name:     "timeout error",
			input:    errors.New("operation timeout"),
			expected: "Operation timed out",
		},
		{
			name:     "not found error",
			input:    errors.New("resource not found"),
			expected: "Requested resource not found",
		},
		{
			name:     "invalid input error",
			input:    errors.New("invalid parameter provided"),
			expected: "Invalid input provided",
		},
		{
			name:     "malformed error",
			input:    errors.New("malformed request body"),
			expected: "Invalid input provided",
		},
		{
			name:     "unknown error",
			input:    errors.New("some other random error"),
			expected: "Internal server error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.sanitizeError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateRequest(t *testing.T) {
	server := &Server{}
	
	// Test valid request
	validReq := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	
	resp := server.validateRequest(validReq)
	assert.Nil(t, resp, "Valid request should return nil")
	
	// Test various invalid cases are covered by TestHandleRequest_ValidationErrors
}

func TestSendResponse_MarshalingError(t *testing.T) {
	server := &Server{}
	
	// Create a response with an unmarshalable field
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  make(chan int), // channels can't be marshaled to JSON
	}
	
	// This should not panic and should handle the error gracefully
	// We can't easily capture stdout in this test, but we can ensure it doesn't panic
	assert.NotPanics(t, func() {
		server.sendResponse(resp)
	})
}